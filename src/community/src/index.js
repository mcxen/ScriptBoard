// ScriptBoard community, a Cloudflare Worker backed by D1.
//
// - Anyone can list and download approved scripts. No login.
// - Uploading needs a login, and the upload waits as "pending".
// - The admin reviews pending scripts with the admin key (the ADMIN_KEY
//   secret), at /admin or through the API.

import adminPage from "./admin.html";

const JSON_HEADERS = { "content-type": "application/json; charset=utf-8" };
const CORS = {
  "access-control-allow-origin": "*",
  "access-control-allow-headers": "authorization, content-type, x-admin-key",
  "access-control-allow-methods": "GET, POST, OPTIONS",
};

const MAX_SOURCE_BYTES = 64 * 1024;
const MAX_UPLOADS_PER_DAY = 10;
const SESSION_DAYS = 30;
const PBKDF2_ITERATIONS = 100000; // the most Workers allow

export default {
  async fetch(request, env) {
    if (request.method === "OPTIONS") {
      return new Response(null, { headers: CORS });
    }
    try {
      return await route(request, env);
    } catch (err) {
      console.error("unhandled", err);
      return fail(500, "服务器出错");
    }
  },
};

async function route(request, env) {
  const url = new URL(request.url);
  const path = url.pathname;
  const method = request.method;

  if (method === "GET" && path === "/") return homePage(env);
  if (method === "GET" && path === "/admin") {
    return new Response(adminPage, { headers: { "content-type": "text/html; charset=utf-8" } });
  }
  if (method === "GET" && path === "/api/health") return json({ ok: true });

  if (method === "POST" && path === "/api/auth/register") return register(request, env);
  if (method === "POST" && path === "/api/auth/login") return login(request, env);
  if (method === "POST" && path === "/api/auth/logout") return logout(request, env);
  if (method === "GET" && path === "/api/me") {
    const user = await requireUser(request, env);
    return user ? json({ user }) : fail(401, "请先登录");
  }

  if (method === "GET" && path === "/api/scripts") return listApproved(env, url);
  if (method === "POST" && path === "/api/scripts") return upload(request, env);
  if (method === "GET" && path === "/api/me/scripts") return myScripts(request, env);

  const download = path.match(/^\/api\/scripts\/([a-f0-9-]{36})\/download$/);
  if (method === "GET" && download) return downloadScript(env, download[1]);

  const one = path.match(/^\/api\/scripts\/([a-f0-9-]{36})$/);
  if (method === "GET" && one) return getApproved(env, one[1]);

  if (method === "GET" && path === "/api/admin/scripts") return adminList(request, env, url);
  const review = path.match(/^\/api\/admin\/scripts\/([a-f0-9-]{36})\/review$/);
  if (method === "POST" && review) return reviewScript(request, env, review[1]);

  return fail(404, "没有这个接口");
}

// ---- accounts -------------------------------------------------------------

async function register(request, env) {
  const body = await readJSON(request);
  if (!body) return fail(400, "请求内容不是 JSON");
  const username = String(body.username ?? "").trim();
  const password = String(body.password ?? "");
  if (!/^[A-Za-z0-9_]{3,20}$/.test(username)) {
    return fail(400, "用户名为 3 到 20 位字母、数字或下划线");
  }
  if (password.length < 8 || password.length > 200) {
    return fail(400, "密码至少 8 位");
  }
  const taken = await env.DB.prepare("SELECT id FROM users WHERE username = ?").bind(username).first();
  if (taken) return fail(409, "用户名已被注册");

  const salt = crypto.getRandomValues(new Uint8Array(16));
  const id = crypto.randomUUID();
  try {
    await env.DB
      .prepare("INSERT INTO users (id, username, pass_hash, pass_salt, created_at) VALUES (?, ?, ?, ?, ?)")
      .bind(id, username, await pbkdf2(password, salt), toBase64(salt), Date.now())
      .run();
  } catch (err) {
    if (String(err).includes("UNIQUE")) return fail(409, "用户名已被注册");
    throw err;
  }
  const token = await issueSession(env, id);
  return json({ token, user: { id, username } }, 201);
}

async function login(request, env) {
  const body = await readJSON(request);
  if (!body) return fail(400, "请求内容不是 JSON");
  const username = String(body.username ?? "").trim();
  const password = String(body.password ?? "");
  const row = await env.DB
    .prepare("SELECT id, username, pass_hash, pass_salt FROM users WHERE username = ?")
    .bind(username)
    .first();
  if (!row) {
    // Spend the same time as a real check, so the reply does not reveal the name.
    await pbkdf2(password, new Uint8Array(16));
    return fail(401, "用户名或密码不对");
  }
  const hash = await pbkdf2(password, fromBase64(row.pass_salt));
  if (!safeEqual(hash, row.pass_hash)) return fail(401, "用户名或密码不对");
  const token = await issueSession(env, row.id);
  return json({ token, user: { id: row.id, username: row.username } });
}

async function logout(request, env) {
  const token = bearer(request);
  if (token) {
    await env.DB.prepare("DELETE FROM sessions WHERE token_hash = ?").bind(await sha256Hex(token)).run();
  }
  return json({ ok: true });
}

async function requireUser(request, env) {
  const token = bearer(request);
  if (!token) return null;
  const row = await env.DB
    .prepare(
      "SELECT u.id, u.username FROM sessions s JOIN users u ON u.id = s.user_id " +
        "WHERE s.token_hash = ? AND s.expires_at > ?",
    )
    .bind(await sha256Hex(token), Date.now())
    .first();
  return row ?? null;
}

async function issueSession(env, userId) {
  const token = toBase64Url(crypto.getRandomValues(new Uint8Array(32)));
  await env.DB
    .prepare("INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)")
    .bind(await sha256Hex(token), userId, Date.now() + SESSION_DAYS * 86400000)
    .run();
  return token;
}

// ---- scripts --------------------------------------------------------------

async function listApproved(env, url) {
  const limit = clamp(parseInt(url.searchParams.get("limit") ?? "30", 10), 1, 100);
  const offset = Math.max(0, parseInt(url.searchParams.get("offset") ?? "0", 10) || 0);
  const rows = await env.DB
    .prepare(
      "SELECT id, name, icon, description, author_name, downloads, created_at " +
        "FROM scripts WHERE status = 'approved' ORDER BY created_at DESC LIMIT ? OFFSET ?",
    )
    .bind(limit, offset)
    .all();
  return json({ scripts: rows.results });
}

async function getApproved(env, id) {
  const row = await env.DB
    .prepare(
      "SELECT id, name, icon, description, author_name, downloads, created_at, source " +
        "FROM scripts WHERE id = ? AND status = 'approved'",
    )
    .bind(id)
    .first();
  return row ? json(row) : fail(404, "没有这个脚本，或还没有通过审核");
}

// The download is the script as the app imports it. Anyone may download
// an approved script without logging in.
async function downloadScript(env, id) {
  const row = await env.DB
    .prepare("SELECT name, icon, source FROM scripts WHERE id = ? AND status = 'approved'")
    .bind(id)
    .first();
  if (!row) return fail(404, "没有这个脚本，或还没有通过审核");
  await env.DB.prepare("UPDATE scripts SET downloads = downloads + 1 WHERE id = ?").bind(id).run();
  return json({ format: "scriptboard/1", name: row.name, icon: row.icon ?? "", source: row.source });
}

// Uploading needs a login. The script waits for the admin to review it.
async function upload(request, env) {
  const user = await requireUser(request, env);
  if (!user) return fail(401, "上传需要先登录");
  const body = await readJSON(request);
  if (!body) return fail(400, "请求内容不是 JSON");

  const name = String(body.name ?? "").trim();
  if (name.length < 1 || name.length > 60) return fail(400, "名称为 1 到 60 个字");
  const description = String(body.description ?? "").trim().slice(0, 500);
  const icon = String(body.icon ?? "").trim();
  if (icon && !/^[a-z-]{1,40}$/.test(icon)) return fail(400, "图标名不对");
  const source = String(body.source ?? "");
  if (new TextEncoder().encode(source).length > MAX_SOURCE_BYTES) {
    return fail(413, "脚本太大，最多 64 KB");
  }
  if (!/export\s+function\s+meta\s*\(/.test(source) || !/export\s+function\s+render\s*\(/.test(source)) {
    return fail(400, "脚本需要导出 meta() 和 render(ctx)");
  }

  const today = await env.DB
    .prepare("SELECT COUNT(*) AS n FROM scripts WHERE author_id = ? AND created_at > ?")
    .bind(user.id, Date.now() - 86400000)
    .first();
  if (today.n >= MAX_UPLOADS_PER_DAY) return fail(429, "今天上传太多了，明天再来");

  const id = crypto.randomUUID();
  await env.DB
    .prepare(
      "INSERT INTO scripts (id, author_id, author_name, name, icon, description, source, status, downloads, created_at) " +
        "VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', 0, ?)",
    )
    .bind(id, user.id, user.username, name, icon, description, source, Date.now())
    .run();
  return json({ id, status: "pending", message: "已提交，等待管理员审核" }, 201);
}

async function myScripts(request, env) {
  const user = await requireUser(request, env);
  if (!user) return fail(401, "请先登录");
  const rows = await env.DB
    .prepare(
      "SELECT id, name, icon, status, downloads, created_at, reviewed_at, review_note " +
        "FROM scripts WHERE author_id = ? ORDER BY created_at DESC LIMIT 100",
    )
    .bind(user.id)
    .all();
  return json({ scripts: rows.results });
}

// ---- admin ----------------------------------------------------------------

function checkAdmin(request, env) {
  if (!env.ADMIN_KEY) return fail(503, "管理员密钥还没有设置（wrangler secret put ADMIN_KEY）");
  const key = request.headers.get("x-admin-key") ?? "";
  if (!key || !safeEqual(key, env.ADMIN_KEY)) return fail(401, "管理员密钥不对");
  return null;
}

async function adminList(request, env, url) {
  const denied = checkAdmin(request, env);
  if (denied) return denied;
  const status = url.searchParams.get("status") ?? "pending";
  if (!["pending", "approved", "rejected"].includes(status)) return fail(400, "状态不对");
  const rows = await env.DB
    .prepare(
      "SELECT id, author_name, name, icon, description, source, status, created_at, review_note " +
        "FROM scripts WHERE status = ? ORDER BY created_at ASC LIMIT 100",
    )
    .bind(status)
    .all();
  return json({ scripts: rows.results });
}

async function reviewScript(request, env, id) {
  const denied = checkAdmin(request, env);
  if (denied) return denied;
  const body = await readJSON(request);
  if (!body) return fail(400, "请求内容不是 JSON");
  const action = body.action;
  if (action !== "approve" && action !== "reject") return fail(400, "action 只能是 approve 或 reject");
  const note = String(body.note ?? "").slice(0, 500);
  const result = await env.DB
    .prepare("UPDATE scripts SET status = ?, reviewed_at = ?, review_note = ? WHERE id = ? AND status = 'pending'")
    .bind(action === "approve" ? "approved" : "rejected", Date.now(), note, id)
    .run();
  if (!result.meta || result.meta.changes === 0) return fail(404, "没有这个待审核的脚本");
  return json({ id, status: action === "approve" ? "approved" : "rejected" });
}

// ---- the public page ------------------------------------------------------

async function homePage(env) {
  const rows = await env.DB
    .prepare(
      "SELECT id, name, description, author_name, downloads FROM scripts " +
        "WHERE status = 'approved' ORDER BY created_at DESC LIMIT 100",
    )
    .all();
  const items = rows.results
    .map(
      (r) =>
        `<li><strong>${escapeHtml(r.name)}</strong> · ${escapeHtml(r.author_name)} · 下载 ${r.downloads}` +
        `<br><span>${escapeHtml(r.description ?? "")}</span>` +
        ` <a href="/api/scripts/${r.id}/download">下载 JSON</a></li>`,
    )
    .join("");
  const html =
    `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
    `<title>ScriptBoard 社区</title><body style="font-family:-apple-system,sans-serif;max-width:640px;margin:24px auto;padding:0 16px">` +
    `<h1>ScriptBoard 社区</h1><p>已审核的脚本，无需登录即可下载。</p><ul style="padding-left:18px;line-height:1.6">${items || "<li>还没有脚本</li>"}</ul>`;
  return new Response(html, { headers: { "content-type": "text/html; charset=utf-8" } });
}

// ---- helpers --------------------------------------------------------------

function json(data, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: { ...JSON_HEADERS, ...CORS } });
}

function fail(status, message) {
  return json({ error: message }, status);
}

async function readJSON(request) {
  try {
    return await request.json();
  } catch {
    return null;
  }
}

function bearer(request) {
  const m = (request.headers.get("authorization") ?? "").match(/^Bearer\s+(\S+)$/i);
  return m ? m[1] : null;
}

function clamp(n, lo, hi) {
  if (Number.isNaN(n)) return lo;
  return Math.min(hi, Math.max(lo, n));
}

async function sha256Hex(text) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function pbkdf2(password, salt) {
  const key = await crypto.subtle.importKey("raw", new TextEncoder().encode(password), "PBKDF2", false, [
    "deriveBits",
  ]);
  const bits = await crypto.subtle.deriveBits(
    { name: "PBKDF2", salt, iterations: PBKDF2_ITERATIONS, hash: "SHA-256" },
    key,
    256,
  );
  return toBase64(new Uint8Array(bits));
}

// Compares two strings in the same time whatever they contain.
function safeEqual(a, b) {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}

function toBase64(bytes) {
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s);
}

function fromBase64(text) {
  const bin = atob(text);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function toBase64Url(bytes) {
  return toBase64(bytes).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}
