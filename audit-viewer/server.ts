import {readdir, readFile} from "fs/promises";
import {join} from "path";
import {homedir} from "os";
import {load} from "js-yaml";
import {Database} from "duckdb";

const PORT = 3000;
const DEFAULT_CONFIG_PATH = join(homedir(), "agents", "config.yaml");

// Config interface
interface Config {
  audit: {
    type?: "file" | "database";
    path?: string;
  };
  db_path?: string;
}

let config: Config;
let db: Database | null = null;
let AUDIT_DIR: string = "";

// Load config
try {
  const configPath = process.env.AGENTS_CONFIG || DEFAULT_CONFIG_PATH;
  console.log(`Loading config from ${configPath}`);
  const configFile = await readFile(configPath, "utf-8");
  config = load(configFile) as Config;

  if (!config.audit) {
    throw new Error("Audit configuration missing from config file");
  }

  if (config.audit.type === "database") {
    if (!config.db_path) {
      throw new Error("Database path (db_path) missing in config");
    }
  } else {
    if (!config.audit.path) {
      throw new Error("Audit path (audit.path) missing in config");
    }
    AUDIT_DIR = config.audit.path.replace(/^~/, homedir());
  }
} catch (e) {
  console.error(`❌ Fatal error loading configuration: ${e}`);
  process.exit(1);
}

// Setup DB if needed
if (config.audit?.type === "database" && config.db_path) {
  let dbPath = config.db_path;
  if (dbPath.startsWith("~")) {
    dbPath = dbPath.replace("~", homedir());
  }

  console.log(`Connecting to database at ${dbPath}`);
  db = new Database(dbPath);
}

// Helper to promisify duckdb all
const dbAll = (query: string, params: any[] = []): Promise<any[]> => {
  return new Promise((resolve, reject) => {
    if (!db) return reject(new Error("Database not initialized"));
    db.all(query, ...params, (err, rows) => {
        console.log("Query result", query, params, err);
      if (err) reject(err);
      else resolve(rows);
    });
  });
};

// Serve static files and API
const server = Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);

    // API endpoint to list all audit files
    if (url.pathname === "/api/audits") {
      try {
        if (config.audit?.type === "database") {
          if (!db) throw new Error("Database not connected");
          const rows = await dbAll(
            "SELECT * FROM audit_sessions ORDER BY created_at DESC"
          );

          const sessions = rows.map((row: any) => {
            const date = new Date(row.created_at);
            return {
              filename: row.id, // Use ID as identifier
              hash: row.session_hash.substring(0, 8),
              fullHash: row.session_hash,
              timestamp: Math.floor(date.getTime() / 1000),
              date: date.toISOString(),
            };
          });
          return new Response(JSON.stringify(sessions), {
            headers: { "Content-Type": "application/json" },
          });
        } else {
          // File mode
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
        }
      } catch (error) {
        return new Response(JSON.stringify({ error: String(error) }), {
          status: 500,
          headers: { "Content-Type": "application/json" },
        });
      }
    }

    // API endpoint to get a specific audit file
    if (url.pathname.startsWith("/api/audit/")) {
      const idOrFilename = url.pathname.replace("/api/audit/", "");
      try {
        if (config.audit?.type === "database") {
          if (!db) throw new Error("Database not connected");

          // Fetch session
          const sessions = await dbAll(
            "SELECT * FROM audit_sessions WHERE id = ?",
              [idOrFilename]
          );
          if (sessions.length === 0) throw new Error("Session not found");
          const session = sessions[0];

          // Fetch events
          const events = await dbAll(
            "SELECT * FROM audit_events WHERE session_id = ? ORDER BY created_at ASC",
            [idOrFilename]
          );

          const responseLines = [];

          // Add synthetic User object (first line in file format)
          responseLines.push({
            id: session.id,
            system_prompt: session.system_prompt,
          });

          // Map events
          events.forEach((row: any) => {
            let payload = null;
            try {
              if (row.payload && row.payload !== "null") {
                payload =
                  typeof row.payload === "string"
                    ? JSON.parse(row.payload)
                    : row.payload;
              }
            } catch (e) {
              console.error("Failed to parse payload", e);
            }

            const event: any = {
              type: row.type,
              content: row.content || "",
            };

            if (payload) {
              if (row.type === "function_call") event.function_call = payload;
              else if (row.type === "function_response")
                event.function_response = payload;
              else if (row.type === "initial_message")
                event.initial_message = payload;
            }

            responseLines.push(event);
          });

          return new Response(JSON.stringify(responseLines), {
            headers: { "Content-Type": "application/json" },
          });
        } else {
          // File mode
          const filePath = join(AUDIT_DIR, idOrFilename);
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
        }
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
console.log(
  `📂 Source: ${
    config.audit?.type === "database" ? "Database" : "File System"
  }`
);
