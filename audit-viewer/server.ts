import {readdir, readFile} from "fs/promises";
import {join} from "path";

const AUDIT_DIR = join(process.cwd(), "..", "audit");
const PORT = 3000;

// Serve static files and API
const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);

    // API endpoint to list all audit files
    if (url.pathname === "/api/audits") {
      try {
        const files = await readdir(AUDIT_DIR);
        const jsonFiles = files
          .filter((f) => f.endsWith(".json"))
          .map((f) => {
            const [hash, timestamp] = f.replace(".json", "").split("_");
            return {
              filename: f,
              hash: hash.substring(0, 8),
              fullHash: hash,
              timestamp: parseInt(timestamp),
              date: new Date(parseInt(timestamp) * 1000).toISOString(),
            };
          })
          .sort((a, b) => b.timestamp - a.timestamp);

        return new Response(JSON.stringify(jsonFiles), {
          headers: { "Content-Type": "application/json" },
        });
      } catch (error) {
        return new Response(JSON.stringify({ error: String(error) }), {
          status: 500,
          headers: { "Content-Type": "application/json" },
        });
      }
    }

    // API endpoint to get a specific audit file
    if (url.pathname.startsWith("/api/audit/")) {
      const filename = url.pathname.replace("/api/audit/", "");
      try {
        const filePath = join(AUDIT_DIR, filename);
        const content = await readFile(filePath, "utf-8");
        
        // Parse JSONL (JSON Lines) format
        const lines = content
          .trim()
          .split("\n")
          .filter((line) => line.trim())
          .map((line) => JSON.parse(line));

        return new Response(JSON.stringify(lines), {
          headers: { "Content-Type": "application/json" },
        });
      } catch (error) {
        return new Response(JSON.stringify({ error: String(error) }), {
          status: 500,
          headers: { "Content-Type": "application/json" },
        });
      }
    }

    // Serve the React app
    if (url.pathname === "/" || url.pathname === "/index.html") {
      return new Response(await readFile(join(import.meta.dir, "index.html")), {
        headers: { "Content-Type": "text/html" },
      });
    }

    // Serve the bundled React app
    if (url.pathname === "/app.tsx") {
      const file = Bun.file(join(import.meta.dir, "app.tsx"));
      const transpiled = await Bun.build({
        entrypoints: [join(import.meta.dir, "app.tsx")],
        format: "esm",
        target: "browser",
      });
      
      if (transpiled.outputs.length > 0) {
        return new Response(transpiled.outputs[0], {
          headers: { "Content-Type": "application/javascript" },
        });
      }
    }

    return new Response("Not Found", { status: 404 });
  },
});

console.log(`🚀 Audit Viewer running at http://localhost:${PORT}`);
