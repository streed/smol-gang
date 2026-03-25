#!/usr/bin/env python3
"""Bridge server between the smol-cluster gateway and smolagent.

Runs alongside smolagent in the agent pod, providing:
- POST /message - forward user messages to the agent
- POST /complete - tell agent to wrap up, commit, push, create PR
- GET /status - agent status
- GET /health - health check
"""

import json
import logging
import os
import subprocess
import sys
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.request import Request, urlopen
from urllib.error import URLError

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("bridge")

GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:8080")
WORKSTREAM_ID = os.environ.get("WORKSTREAM_ID", "")
ACP_PORT = int(os.environ.get("ACP_PORT", "8021"))
BRIDGE_PORT = int(os.environ.get("BRIDGE_PORT", "8022"))

agent_status = "starting"
agent_lock = threading.Lock()


def report_to_gateway(endpoint: str, data: dict):
    """Send data back to the gateway."""
    url = f"{GATEWAY_URL}/api/v1/internal/workstreams/{WORKSTREAM_ID}/{endpoint}"
    try:
        req = Request(
            url,
            data=json.dumps(data).encode(),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urlopen(req, timeout=10) as resp:
            return resp.status == 200
    except (URLError, Exception) as e:
        logger.error(f"Failed to report to gateway: {e}")
        return False


def send_to_acp(message: str) -> str:
    """Send a message to the smolagent ACP interface."""
    try:
        data = json.dumps({"message": message}).encode()
        req = Request(
            f"http://localhost:{ACP_PORT}/message",
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urlopen(req, timeout=300) as resp:
            return resp.read().decode()
    except Exception as e:
        logger.error(f"Failed to send to ACP: {e}")
        return json.dumps({"error": str(e)})


def run_git_push_and_pr() -> str:
    """Push changes and create a PR."""
    branch = os.environ.get("BRANCH_NAME", "")
    repo_owner = os.environ.get("GITHUB_OWNER", "")
    repo_name = os.environ.get("GITHUB_REPO", "")

    try:
        # Stage and commit any remaining changes
        subprocess.run(["git", "add", "-A"], cwd="/workspace/repo", check=False)
        subprocess.run(
            ["git", "commit", "-m", "chore: final agent changes"],
            cwd="/workspace/repo",
            check=False,
        )

        # Push
        subprocess.run(
            ["git", "push", "-u", "origin", branch],
            cwd="/workspace/repo",
            check=True,
        )

        # Create PR using gh CLI if available, otherwise report URL pattern
        pr_title = f"[smol-agent] {branch}"
        try:
            result = subprocess.run(
                ["gh", "pr", "create", "--title", pr_title, "--body",
                 "Automated PR created by smol-cluster agent", "--base", "main"],
                cwd="/workspace/repo",
                capture_output=True,
                text=True,
                check=True,
            )
            return result.stdout.strip()
        except (subprocess.CalledProcessError, FileNotFoundError):
            return f"https://github.com/{repo_owner}/{repo_name}/compare/{branch}"

    except subprocess.CalledProcessError as e:
        logger.error(f"Git operation failed: {e}")
        return ""


class BridgeHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            self._respond(200, {"status": "ok"})
        elif self.path == "/status":
            with agent_lock:
                self._respond(200, {"status": agent_status})
        else:
            self._respond(404, {"error": "not found"})

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length) if content_length > 0 else b"{}"

        try:
            data = json.loads(body)
        except json.JSONDecodeError:
            self._respond(400, {"error": "invalid JSON"})
            return

        if self.path == "/message":
            self._handle_message(data)
        elif self.path == "/complete":
            self._handle_complete(data)
        else:
            self._respond(404, {"error": "not found"})

    def _handle_message(self, data: dict):
        global agent_status
        content = data.get("content", "")
        if not content:
            self._respond(400, {"error": "content is required"})
            return

        with agent_lock:
            agent_status = "processing"

        # Forward to ACP
        response = send_to_acp(content)

        with agent_lock:
            agent_status = "idle"

        # Report agent response back to gateway
        report_to_gateway("agent-message", {"content": response})

        self._respond(200, {"response": response})

    def _handle_complete(self, data: dict):
        global agent_status
        with agent_lock:
            agent_status = "completing"

        # Push and create PR
        pr_url = run_git_push_and_pr()

        # Report status
        report_to_gateway("status", {
            "status": "completed",
            "pull_request_url": pr_url,
        })

        with agent_lock:
            agent_status = "completed"

        self._respond(200, {"pull_request_url": pr_url})

    def _respond(self, status: int, body: dict):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def log_message(self, format, *args):
        logger.info(f"{self.client_address[0]} - {format % args}")


def main():
    global agent_status

    server = HTTPServer(("0.0.0.0", BRIDGE_PORT), BridgeHandler)
    logger.info(f"Bridge server listening on port {BRIDGE_PORT}")

    with agent_lock:
        agent_status = "idle"

    report_to_gateway("status", {"status": "running"})

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Bridge server shutting down")
        server.shutdown()


if __name__ == "__main__":
    main()
