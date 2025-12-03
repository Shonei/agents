import { serve, file } from "bun";
import { readdir } from "node:fs/promises";
import { join } from "path";

// Go up one level from audit-ui to find audit folder
const AUDIT_DIR = join(process.cwd(), "..", "audit");

console.log("Serving audits from:", AUDIT_DIR);

serve({
  port: 3000,
  async fetch(req) {
    const url = new URL(req.url);

    // CORS headers for dev
    const headers = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET",
      "Content-Type": "application/json"
    };

    if (url.pathname === "/api/audits") {
      try {
        const files = await readdir(AUDIT_DIR);
        // Filter for .json files
        const jsonFiles = files.filter(f => f.endsWith(".json"));
        return new Response(JSON.stringify(jsonFiles), { headers });
      } catch (e) {
        console.error(e);
        return new Response(JSON.stringify({ error: "Error reading directory" }), { status: 500, headers });
      }
    }

    if (url.pathname.startsWith("/api/audits/")) {
      const filename = url.pathname.replace("/api/audits/", "");
      if (!filename || filename.includes("..")) return new Response("Not found", { status: 404, headers });
      
      try {
        const filePath = join(AUDIT_DIR, filename);
        const f = file(filePath);
        if (await f.exists()) {
           const content = await f.text();
           // Parse JSONL
           const lines = content.trim().split("\n")
             .filter(line => line.trim() !== "")
             .map(line => {
               try { return JSON.parse(line); } catch(e) { return null; }
             })
             .filter(x => x !== null);
           
           return new Response(JSON.stringify(lines), { headers });
        }
        return new Response("Not found", { status: 404, headers });
      } catch (e) {
        console.error(e);
        return new Response(JSON.stringify({ error: "Error reading file" }), { status: 500, headers });
      }
    }
    
    return new Response("Not found", { status: 404, headers });
  },
});

console.log("Backend running on http://localhost:3000");
