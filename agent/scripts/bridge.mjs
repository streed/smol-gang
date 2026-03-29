#!/usr/bin/env node
import { spawn, execSync } from 'node:child_process';
import http from 'node:http';
import { Writable, Readable } from 'node:stream';
import { createRequire } from 'node:module';

// Load ACP SDK from smol-agent's dependencies
const require = createRequire('/opt/smol-agent/node_modules/');
const acp = await import('/opt/smol-agent/node_modules/@agentclientprotocol/sdk/dist/acp.js');

// Config from env
const config = {
  port: parseInt(process.env.BRIDGE_PORT || '8022'),
  gatewayUrl: process.env.GATEWAY_URL || 'http://localhost:8080',
  workstreamId: process.env.WORKSTREAM_ID || '',
  gatewayToken: process.env.GATEWAY_TOKEN || '',
  workspace: process.env.WORKSPACE || '/workspace/repo',
  branch: process.env.BRANCH_NAME || '',
  llmProvider: process.env.LLM_PROVIDER || 'ollama',
  llmModel: process.env.LLM_MODEL || '',
  llmApiKey: process.env.LLM_API_KEY || '',
  llmApiUrl: process.env.LLM_API_URL || '',
  baseBranch: process.env.BASE_BRANCH || 'main',
};

let connection = null;
let sessionId = null;
let agentProcess = null;
let agentStatus = 'starting';
let prCreated = false;

// ── Gateway Communication ──────────────────────────────────────────

// Filter out system prompt noise and internal agent output
function shouldFilter(content) {
  const c = String(content).trim();
  if (!c) return true;
  if (c.startsWith('[status:null]') || c.startsWith('[status:running]')) return true;
  // Filter system prompt fragments
  if (c.startsWith('You are a') && c.includes('coding agent')) return true;
  if (c.startsWith('You are an AI') || c.startsWith('You are a helpful')) return true;
  if (c.includes('IMPORTANT RULES') || c.includes('TOOL GUIDELINES')) return true;
  if (c.includes('Available tools:') && c.length > 500) return true;
  // Filter raw diff output that leaks through
  if (c.startsWith('diff ') && c.includes('---') && c.includes('+++')) return true;
  if (c.startsWith('--- a/') || c.startsWith('+++ b/')) return true;
  if (/^@@\s/.test(c)) return true;
  // Filter ANSI escape sequences only output
  if (/^[\x1b\[\]0-9;mKhldA-Z\s?]*$/.test(c)) return true;
  return false;
}

function reportToGateway(content) {
  // Strip ANSI escape codes and filter noise
  const cleaned = String(content).replace(/\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07?|\[[\?0-9]*[hlK]/g, '').trim();
  if (shouldFilter(cleaned)) return;
  const url = `${config.gatewayUrl}/api/v1/internal/workstreams/${config.workstreamId}/agent-message`;
  const data = JSON.stringify({ source: 'agent', content: cleaned });
  const req = http.request(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(data),
    },
  });
  req.on('error', (e) => console.error(`[bridge] Gateway report error: ${e.message}`));
  req.write(data);
  req.end();
}

// ── ACP Client ─────────────────────────────────────────────────────

// Accumulates text chunks and flushes as complete paragraphs/messages
let textBuffer = '';
let flushTimer = null;

function flushTextBuffer() {
  if (flushTimer) { clearTimeout(flushTimer); flushTimer = null; }
  if (textBuffer.trim()) {
    reportToGateway(textBuffer.trim());
    textBuffer = '';
  }
}

function appendText(text) {
  textBuffer += text;

  // Flush on paragraph breaks (double newline) or when buffer is large
  if (textBuffer.includes('\n\n')) {
    const parts = textBuffer.split('\n\n');
    // Send all complete paragraphs, keep the last (possibly incomplete) one
    for (let i = 0; i < parts.length - 1; i++) {
      const part = parts[i].trim();
      if (part) reportToGateway(part);
    }
    textBuffer = parts[parts.length - 1];
  }

  if (textBuffer.length > 800) {
    flushTextBuffer();
    return;
  }

  // Reset the idle timer - flush after 2s of silence
  if (flushTimer) clearTimeout(flushTimer);
  flushTimer = setTimeout(flushTextBuffer, 2000);
}

class BridgeClient {
  async requestPermission(params) {
    // Auto-approve everything (yolo mode)
    const option = params.options.find(o => o.kind === 'allow_once') || params.options[0];
    return { outcome: { outcome: 'selected', optionId: option.optionId } };
  }

  async sessionUpdate(params) {
    const update = params.update;
    switch (update.sessionUpdate) {
      case 'agent_message_chunk':
        if (update.content?.type === 'text' && update.content.text) {
          appendText(update.content.text);
          agentResponseText += update.content.text;
        }
        break;
      case 'tool_call': {
        flushTextBuffer();
        const name = update.name || update.title || 'unknown';
        let args = {};
        try {
          args = typeof update.args === 'object' && update.args ? update.args : {};
        } catch { args = {}; }

        let detail = `🔧 **${name}**`;
        if (typeof args.command === 'string') detail += `\n\`\`\`\n${args.command.slice(0, 200)}\n\`\`\``;
        else if (typeof args.filePath === 'string') detail += `: \`${args.filePath}\``;
        else if (typeof args.path === 'string') detail += `: \`${args.path}\``;
        else if (typeof args.pattern === 'string') detail += `: \`${args.pattern}\``;
        else if (typeof args.args === 'string') detail += `: \`${args.args.slice(0, 100)}\``;
        reportToGateway(detail);
        break;
      }
      case 'tool_call_update': {
        if (update.status === 'completed' && update.output) {
          // Only show concise output — skip large diffs and file contents
          const output = String(update.output);
          if (output.length > 1000 || output.includes('diff --git') || output.includes('@@')) {
            // Skip large/diff output — the Changes tab shows this better
          } else if (output.trim()) {
            reportToGateway(`📋 \`\`\`\n${output.slice(0, 300).trim()}\n\`\`\``);
          }
        } else if (update.status === 'error') {
          const errMsg = typeof update.error === 'string' ? update.error : String(update.error || update.status);
          reportToGateway(`❌ ${errMsg.slice(0, 300)}`);
        }
        break;
      }
      case 'agent_thought_chunk':
        if (update.content?.type === 'text' && update.content.text) {
          appendText(`💭 ${update.content.text}`);
        }
        break;
      case 'plan':
        if (update.content) {
          flushTextBuffer();
          reportToGateway(`📝 Plan:\n${typeof update.content === 'string' ? update.content : JSON.stringify(update.content)}`);
        }
        break;
    }
  }

  async writeTextFile() { return {}; }
  async readTextFile() { return { content: '' }; }
}

async function startAgent() {
  const args = ['--acp', '--auto-approve', '-d', config.workspace];

  if (config.llmProvider) args.push('--provider', config.llmProvider);
  if (config.llmModel) args.push('--model', config.llmModel);
  if (config.llmApiKey) {
    args.push('--api-key', config.llmApiKey);
  }
  if (config.llmApiUrl) args.push('--host', config.llmApiUrl);

  console.log(`[bridge] Starting smol-agent: ${args.join(' ')}`);

  agentProcess = spawn('smol-agent', args, {
    stdio: ['pipe', 'pipe', 'inherit'],
    cwd: config.workspace,
  });

  agentProcess.on('exit', (code) => {
    console.error(`[bridge] smol-agent exited with code ${code}`);
    agentStatus = 'crashed';
  });

  // Wait for process to start
  await new Promise(r => setTimeout(r, 1500));
  if (agentProcess.exitCode !== null) {
    throw new Error(`smol-agent failed to start (exit code ${agentProcess.exitCode})`);
  }

  // Create ACP connection using the SDK
  const input = Writable.toWeb(agentProcess.stdin);
  const output = Readable.toWeb(agentProcess.stdout);
  const stream = acp.ndJsonStream(input, output);
  const client = new BridgeClient();
  connection = new acp.ClientSideConnection((_agent) => client, stream);

  // Initialize
  console.log('[bridge] Initializing ACP...');
  const initResult = await connection.initialize({
    protocolVersion: acp.PROTOCOL_VERSION,
    clientCapabilities: {},
  });
  console.log(`[bridge] ACP initialized (protocol v${initResult.protocolVersion})`);

  // Create session
  console.log('[bridge] Creating session...');
  const sessionResult = await connection.newSession({
    cwd: config.workspace,
    mcpServers: [],
  });
  sessionId = sessionResult.sessionId;
  console.log(`[bridge] Session created: ${sessionId}`);

  agentStatus = 'idle';
  reportToGateway('[status:running] Agent is online and ready');
}

// Queue for serializing prompts — ACP only handles one at a time
const promptQueue = [];
let promptRunning = false;
let agentResponseText = ''; // Captures agent's reasoning during a prompt

async function sendPrompt(text) {
  if (!sessionId || !connection) throw new Error('No active session');

  return new Promise((resolve, reject) => {
    promptQueue.push({ text, resolve, reject });
    processQueue();
  });
}

async function processQueue() {
  if (promptRunning || promptQueue.length === 0) return;
  promptRunning = true;

  const { text, resolve, reject } = promptQueue.shift();
  agentStatus = 'processing';
  agentResponseText = ''; // Reset for this prompt

  try {
    const result = await connection.prompt({
      sessionId,
      prompt: [{ type: 'text', text }],
    });

    flushTextBuffer();

    // Auto-commit, push, and create PR after each prompt
    try {
      const status = execSync('git status --porcelain', { cwd: config.workspace, encoding: 'utf-8' }).trim();
      if (status) {
        execSync('git add -A', { cwd: config.workspace, timeout: 10000 });
        const diffStat = execSync('git diff --cached --stat', { cwd: config.workspace, encoding: 'utf-8', timeout: 5000 }).trim();
        const files = diffStat ? diffStat.split('\n').slice(0, -1).map(l => l.trim().split('|')[0].trim()).join(', ').slice(0, 150) : 'changes';
        const commitHash = (() => { try { return execSync('git rev-parse --short HEAD', { cwd: config.workspace, encoding: 'utf-8' }).trim(); } catch { return ''; } })();
        execSync(`git commit -m "agent: ${files}"`, { cwd: config.workspace, timeout: 10000 });
        execSync(`git push -u origin ${config.branch} 2>&1 || true`, { cwd: config.workspace, timeout: 30000 });
        console.log('[bridge] Auto-committed and pushed changes');

        // Auto-create PR if one doesn't exist yet
        if (!prCreated) {
          const prUrl = await gitPushAndPR();
          if (prUrl && !prUrl.includes('/compare/')) {
            prCreated = true;
          }
        }

        // Leave a summary comment on the PR
        if (prCreated) {
          await postPRComment(diffStat, text);
        }
      }
    } catch (e) {
      console.error(`[bridge] Auto-commit/push error: ${e.message}`);
    }

    agentStatus = 'idle';
    resolve(result.stopReason);
  } catch (err) {
    agentStatus = 'idle';
    const errMsg = `Agent error: ${err.message}`;
    reportToGateway(errMsg);
    resolve(errMsg);
  } finally {
    promptRunning = false;
    // Process next in queue
    if (promptQueue.length > 0) processQueue();
  }
}

// ── Git Operations ────────────────────────────────────────────────

async function gitPushAndPR() {
  const { workspace, branch } = config;

  // Generate a summary commit message from the git diff
  let commitMsg = 'chore: agent changes';
  try {
    const diff = execSync('git diff --stat HEAD', { cwd: workspace, encoding: 'utf-8' }).trim();
    if (diff) {
      const files = diff.split('\n').slice(0, -1).map(l => l.trim().split('|')[0].trim()).join(', ');
      commitMsg = `feat: agent changes to ${files}`.slice(0, 200);
    }
  } catch { /* ignore */ }

  try {
    execSync('git add -A', { cwd: workspace });
    execSync(`git commit -m "${commitMsg.replace(/"/g, '\\"')}"`, { cwd: workspace });
  } catch { /* nothing to commit */ }

  try {
    execSync(`git push -u origin ${branch}`, { cwd: workspace });
  } catch (e) {
    console.error(`[bridge] Git push failed: ${e.message}`);
    return '';
  }

  // Create PR via GitHub API targeting the plan's base branch
  const baseBranch = config.baseBranch;
  const owner = process.env.GITHUB_OWNER || '';
  const repoName = process.env.GITHUB_REPO || '';
  const gitToken = process.env.GIT_TOKEN || '';

  if (!owner || !repoName || !gitToken) {
    console.error('[bridge] Missing GITHUB_OWNER, GITHUB_REPO, or GIT_TOKEN for PR creation');
    return `https://github.com/${owner}/${repoName}/compare/${baseBranch}...${branch}`;
  }

  try {
    const resp = await fetch(`https://api.github.com/repos/${owner}/${repoName}/pulls`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${gitToken}`,
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      body: JSON.stringify({
        title: `[smol-agent] ${branch}`,
        body: `Automated PR by smol-gang agent.\n\nBranch: \`${branch}\`\nBase: \`${baseBranch}\``,
        head: branch,
        base: baseBranch,
      }),
    });

    const data = await resp.json();
    if (data.html_url) {
      console.log(`[bridge] PR created: ${data.html_url}`);
      reportToGateway(`📬 PR opened: ${data.html_url}`);
      return data.html_url;
    } else {
      console.error(`[bridge] PR creation failed (${resp.status}): ${JSON.stringify(data).slice(0, 300)}`);
      return `https://github.com/${owner}/${repoName}/compare/${baseBranch}...${branch}`;
    }
  } catch (e) {
    console.error(`[bridge] PR creation error: ${e.message}`);
    return `https://github.com/${owner}/${repoName}/compare/${baseBranch}...${branch}`;
  }
}

// Post a summary comment on the PR describing what was changed.
// If the prompt contains a smol-gang-meta tag with a comment_id, reply in-thread.
let cachedPRNumber = null;

function parseMeta(prompt) {
  const match = prompt.match(/<!-- smol-gang-meta:\s*pr=(\d+)\s*comment_id=(\d+)(?:\s*type=(\w+))?\s*-->/);
  if (match) {
    return { pr: parseInt(match[1]), commentId: parseInt(match[2]), type: match[3] || 'review_comment' };
  }
  return null;
}

async function findPRNumber() {
  if (cachedPRNumber) return cachedPRNumber;
  const owner = process.env.GITHUB_OWNER || '';
  const repoName = process.env.GITHUB_REPO || '';
  const gitToken = process.env.GIT_TOKEN || '';
  if (!owner || !repoName || !gitToken) return null;

  try {
    const resp = await fetch(
      `https://api.github.com/repos/${owner}/${repoName}/pulls?head=${owner}:${config.branch}&state=open`,
      { headers: { 'Authorization': `Bearer ${gitToken}`, 'Accept': 'application/json' } }
    );
    const prs = await resp.json();
    if (Array.isArray(prs) && prs.length > 0) {
      cachedPRNumber = prs[0].number;
    }
  } catch { /* ignore */ }
  return cachedPRNumber;
}

async function postPRComment(diffStat, prompt) {
  const owner = process.env.GITHUB_OWNER || '';
  const repoName = process.env.GITHUB_REPO || '';
  const gitToken = process.env.GIT_TOKEN || '';
  if (!owner || !repoName || !gitToken) return;

  const commitHash = (() => {
    try { return execSync('git rev-parse --short HEAD', { cwd: config.workspace, encoding: 'utf-8' }).trim(); }
    catch { return ''; }
  })();

  // Use the agent's actual reasoning as the summary
  const summary = agentResponseText.trim().slice(0, 2000);

  let body = '### 🤖 Agent Update\n\n';
  if (summary) {
    body += summary + '\n\n';
  }
  if (diffStat) {
    body += '**Files changed:**\n```\n' + diffStat + '\n```\n';
  }
  if (commitHash) {
    body += `\nCommit: \`${commitHash}\`\n`;
  }

  try {
    if (meta && meta.commentId && meta.type !== 'issue_comment') {
      // Reply in-thread to the review comment
      const prNum = meta.pr || await findPRNumber();
      if (prNum) {
        await fetch(`https://api.github.com/repos/${owner}/${repoName}/pulls/${prNum}/comments/${meta.commentId}/replies`, {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${gitToken}`, 'Content-Type': 'application/json' },
          body: JSON.stringify({ body }),
        });
        console.log(`[bridge] Replied to review comment ${meta.commentId} on PR #${prNum}`);
        return;
      }
    }

    // Fall back to a top-level PR comment
    const prNum = meta?.pr || await findPRNumber();
    if (prNum) {
      await fetch(`https://api.github.com/repos/${owner}/${repoName}/issues/${prNum}/comments`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${gitToken}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ body }),
      });
      console.log(`[bridge] Posted comment on PR #${prNum}`);
    }
  } catch (e) {
    console.error(`[bridge] Failed to post PR comment: ${e.message}`);
  }
}

// ── HTTP Server ───────────────────────────────────────────────────

function parseBody(req) {
  return new Promise(resolve => {
    let body = '';
    req.on('data', chunk => { body += chunk; });
    req.on('end', () => {
      try { resolve(JSON.parse(body || '{}')); } catch { resolve({}); }
    });
  });
}

function respond(res, status, data) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(data));
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    respond(res, 200, { status: agentProcess && agentProcess.exitCode === null ? 'ok' : 'error' });
  } else if (req.method === 'GET' && req.url === '/status') {
    respond(res, 200, { status: agentStatus });
  } else if (req.method === 'POST' && req.url === '/message') {
    const data = await parseBody(req);
    const content = data.content || data.message || '';
    if (!content) return respond(res, 400, { error: 'content or message is required' });
    respond(res, 200, { status: 'accepted' });
    sendPrompt(content).catch(e => console.error(`[bridge] Prompt error: ${e.message}`));
  } else if (req.method === 'GET' && req.url === '/diff') {
    let diff = '', stat = '';
    try {
      // Find the merge base with main/master
      let base = '';
      try {
        base = execSync('git merge-base origin/main HEAD 2>/dev/null', { cwd: config.workspace, encoding: 'utf-8', timeout: 5000 }).trim();
      } catch {
        try {
          base = execSync('git merge-base origin/master HEAD 2>/dev/null', { cwd: config.workspace, encoding: 'utf-8', timeout: 5000 }).trim();
        } catch {
          base = '';
        }
      }

      // Committed changes since base (all commits on this branch)
      let committedDiff = '', committedStat = '';
      if (base) {
        committedDiff = execSync(`git diff ${base} HEAD 2>/dev/null || true`, { cwd: config.workspace, encoding: 'utf-8', timeout: 30000 }).trim();
        committedStat = execSync(`git diff --stat ${base} HEAD 2>/dev/null || true`, { cwd: config.workspace, encoding: 'utf-8', timeout: 10000 }).trim();
      }

      // Uncommitted changes (working tree + staged)
      const uncommittedDiff = execSync('git diff HEAD 2>/dev/null || true', { cwd: config.workspace, encoding: 'utf-8', timeout: 30000 }).trim();
      const stagedDiff = execSync('git diff --cached 2>/dev/null || true', { cwd: config.workspace, encoding: 'utf-8', timeout: 30000 }).trim();

      // Combine: committed first, then uncommitted
      const parts = [committedDiff, stagedDiff, uncommittedDiff].filter(Boolean);
      diff = parts.join('\n');

      // Stat: use committed stat, append uncommitted if any
      stat = committedStat;
      const uncommittedStat = execSync('git diff --stat HEAD 2>/dev/null || true', { cwd: config.workspace, encoding: 'utf-8', timeout: 10000 }).trim();
      if (uncommittedStat) {
        stat = stat ? stat + '\n\n--- Working tree ---\n' + uncommittedStat : uncommittedStat;
      }
    } catch {
      try {
        stat = execSync('git status --short', { cwd: config.workspace, encoding: 'utf-8' });
      } catch { /* ignore */ }
    }
    respond(res, 200, { stat: stat.trim(), diff: diff.trim() });
  } else if (req.method === 'POST' && req.url === '/complete') {
    agentStatus = 'completing';
    const prUrl = await gitPushAndPR();
    agentStatus = 'completed';
    respond(res, 200, { pull_request_url: prUrl });
  } else {
    respond(res, 404, { error: 'not found' });
  }
});

// ── Main ──────────────────────────────────────────────────────────

try {
  await startAgent();
} catch (err) {
  console.error(`[bridge] Failed to start agent: ${err.message}`);
  agentStatus = 'error';
  reportToGateway(`[status:error] Failed to start agent: ${err.message}`);
}

server.listen(config.port, '0.0.0.0', () => {
  console.log(`[bridge] HTTP server listening on port ${config.port}`);
});
