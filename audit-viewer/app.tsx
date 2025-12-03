import React, { useState, useEffect } from "react";
import { createRoot } from "react-dom/client";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface AuditFile {
  filename: string;
  hash: string;
  fullHash: string;
  timestamp: number;
  date: string;
}

interface AuditMessage {
  id?: string;
  type: string;
  content?: string;
  system_prompt?: string;
  function_call?: {
    name: string;
    input: any;
  };
  function_response?: {
    name: string;
    response: string;
  };
}

function App() {
  const [audits, setAudits] = useState<AuditFile[]>([]);
  const [selectedAudit, setSelectedAudit] = useState<string | null>(null);
  const [auditContent, setAuditContent] = useState<AuditMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load audit files list
  useEffect(() => {
    fetch("/api/audits")
      .then((res) => res.json())
      .then((data) => setAudits(data))
      .catch((err) => setError(err.message));
  }, []);

  // Load specific audit content
  const loadAudit = async (filename: string) => {
    setLoading(true);
    setError(null);
    setSelectedAudit(filename);

    try {
      const res = await fetch(`/api/audit/${filename}`);
      const data = await res.json();
      setAuditContent(data);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container">
      <aside className="sidebar">
        <h1>📋 Audits</h1>
        <ul className="audit-list">
          {audits.map((audit) => (
            <li
              key={audit.filename}
              className={`audit-item ${selectedAudit === audit.filename ? "active" : ""}`}
              onClick={() => loadAudit(audit.filename)}
            >
              <div className="audit-hash">{audit.hash}</div>
              <div className="audit-date">
                {new Date(audit.date).toLocaleString()}
              </div>
            </li>
          ))}
        </ul>
      </aside>

      <main className="main-content">
        {loading && <div className="loading">Loading...</div>}
        {error && <div className="error">Error: {error}</div>}
        {!selectedAudit && !loading && (
          <div className="empty-state">
            <h2>👈 Select an audit to review</h2>
            <p>Choose an audit from the sidebar to view its contents</p>
          </div>
        )}
        {!loading && selectedAudit && (
          <AuditContent messages={auditContent} />
        )}
      </main>
    </div>
  );
}

function AuditContent({ messages }: { messages: AuditMessage[] }) {
  return (
    <div>
      {messages.map((msg, idx) => (
        <MessageBlock key={idx} message={msg} index={idx} />
      ))}
    </div>
  );
}

function MessageBlock({ message, index }: { message: AuditMessage; index: number }) {
  // First message contains system prompt and ID
  if (index === 0 && message.id) {
    return (
      <div className="message-block system">
        <div className="message-type">System Initialization</div>
        <div className="message-content">
          <div style={{ marginBottom: "12px", opacity: 0.7 }}>
            <strong>Session ID:</strong> <code>{message.id}</code>
          </div>
          <div className="markdown">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {message.system_prompt || ""}
            </ReactMarkdown>
          </div>
        </div>
      </div>
    );
  }

  // User messages (initial or subsequent)
  if (message.type === "initial_message" || message.type === "user_message") {
    return (
      <div className="message-block user">
        <div className="message-type">User Message</div>
        <div className="message-content markdown">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>
            {message.content || ""}
          </ReactMarkdown>
        </div>
      </div>
    );
  }

  // Assistant message
  if (message.type === "assistant_message") {
    return (
      <div className="message-block assistant">
        <div className="message-type">Assistant</div>
        <div className="message-content markdown">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>
            {message.content || ""}
          </ReactMarkdown>
        </div>
      </div>
    );
  }

  // Function call
  if (message.type === "function_call" && message.function_call) {
    return (
      <div className="message-block function">
        <div className="message-type">Function Call: {message.function_call.name}</div>
        <div className="message-content">
          <div className="json-block">
            {JSON.stringify(message.function_call.input, null, 2)}
          </div>
        </div>
      </div>
    );
  }

  // Function response
  if (message.type === "function_response" && message.function_response) {
    let responseContent = message.function_response.response;
    let isJSON = false;

    // Try to parse and format JSON response
    try {
      const parsed = JSON.parse(responseContent);
      responseContent = JSON.stringify(parsed, null, 2);
      isJSON = true;
    } catch {
      // Not JSON, might be XML or text with tags
    }

    return (
      <div className="message-block function">
        <div className="message-type">Function Response: {message.function_response.name}</div>
        <div className="message-content">
          {isJSON ? (
            <div className="json-block">{responseContent}</div>
          ) : (
            <div className="markdown">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {responseContent}
              </ReactMarkdown>
            </div>
          )}
        </div>
      </div>
    );
  }

  // Fallback for unknown message types
  return (
    <div className="message-block">
      <div className="message-type">{message.type || "Unknown"}</div>
      <div className="message-content">
        <pre>{JSON.stringify(message, null, 2)}</pre>
      </div>
    </div>
  );
}

// Mount the app
const root = createRoot(document.getElementById("root")!);
root.render(<App />);
