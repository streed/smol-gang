#!/usr/bin/env python3
"""Bridge server between the smol-gang gateway and smolagent.

Runs alongside smolagent in the agent pod, providing:
- POST /message - forward user messages to the agent
- POST /complete - tell agent to wrap up, commit, push, create PR
- GET /status - agent status
- GET /health - health check
"""

import argparse
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

# Global config — populated from CLI args or env vars in main()
config = {
    "gateway_url": "",
    "workstream_id": "",
    "gateway_token": "",
    "acp_port": 8021,
    "bridge_port": 8022,
    "workspace": "/workspace/repo",
    "branch": "",
}

agent_status = "starting"
agent_lock = threading.Lock()


def report_to_gateway(endpoint: str, data: dict):
    """Send data back to the gateway."""
    url = f"{config['gateway_url']}/api/v1/internal/workstreams/{config['workstream_id']}/{endpoint}"
    try:
        headers = {"Content-Type": "application/json"}
        if config["gateway_token"]:
            headers["Authorization"] = f"Bearer {config['gateway_token']}"
        req = Request(
            url,
            data=json.dumps(data).encode(),
            headers=headers,
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
            f"http://localhost:{config['acp_port']}/message",
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
    branch = config["branch"]
    workspace = config["workspace"]
    repo_owner = os.environ.get("GITHUB_OWNER", "")
    repo_name = os.environ.get("GITHUB_REPO", "")

    try:
        # Stage and commit any remaining changes
        subprocess.run(["git", "add", "-A"], cwd=workspace, check=False)
        subprocess.run(
            ["git", "commit", "-m", "chore: final agent changes"],
            cwd=workspace,
            check=False,
        )

        # Push
        subprocess.run(
            ["git", "push", "-u", "origin", branch],
            cwd=workspace,
            check=True,
        )

        # Create PR using gh CLI if available, otherwise report URL pattern
        pr_title = f"[smol-agent] {branch}"
        try:
            result = subprocess.run(
                ["gh", "pr", "create", "--title", pr_title, "--body",
                 "Automated PR created by smol-gang agent", "--base", "main"],
                cwd=workspace,
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
        # Accept both "content" and "message" keys for compatibility
        content = data.get("content") or data.get("message", "")
        if not content:
            self._respond(400, {"error": "content or message is required"})
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


def parse_args():
    parser = argparse.ArgumentParser(description="smol-gang bridge server")
    parser.add_argument("--port", type=int, default=int(os.environ.get("BRIDGE_PORT", "8022")),
                        help="Bridge server port")
    parser.add_argument("--acp-port", type=int, default=int(os.environ.get("ACP_PORT", "8021")),
                        help="smolagent ACP port")
    parser.add_argument("--gateway-url", default=os.environ.get("GATEWAY_URL", "http://localhost:8080"),
                        help="Gateway URL")
    parser.add_argument("--workstream-id", default=os.environ.get("WORKSTREAM_ID", ""),
                        help="Workstream ID")
    parser.add_argument("--gateway-token", default=os.environ.get("GATEWAY_TOKEN", ""),
                        help="Gateway auth token")
    parser.add_argument("--workspace", default=os.environ.get("WORKSPACE", "/workspace/repo"),
                        help="Workspace directory")
    parser.add_argument("--branch", default=os.environ.get("BRANCH_NAME", ""),
                        help="Git branch name")
    return parser.parse_args()


def main():
    global agent_status

    args = parse_args()
    config["gateway_url"] = args.gateway_url
    config["workstream_id"] = args.workstream_id
    config["gateway_token"] = args.gateway_token
    config["acp_port"] = args.acp_port
    config["bridge_port"] = args.port
    config["workspace"] = args.workspace
    config["branch"] = args.branch

    server = HTTPServer(("0.0.0.0", config["bridge_port"]), BridgeHandler)
    logger.info(f"Bridge server listening on port {config['bridge_port']}")

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
