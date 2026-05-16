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

// Parsed structure for tool responses
interface ParsedToolResponse {
  type: "structured" | "json" | "text";
  metadata?: Record<string, string>;
  content?: string;
  raw: string;
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
    return <FunctionCallBlock functionCall={message.function_call} />;
  }

  // Function response
  if (message.type === "function_response" && message.function_response) {
    return <FunctionResponseBlock functionResponse={message.function_response} />;
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

function FunctionCallBlock({ functionCall }: { functionCall: { name: string; input: any } }) {
  const [collapsed, setCollapsed] = useState(false);
  const inputStr = JSON.stringify(functionCall.input, null, 2);
  const paramCount = Object.keys(functionCall.input).length;
  const inputSize = new Blob([inputStr]).size;
  const paramNames = Object.keys(functionCall.input);
  const paramPreview = paramNames.length > 0 
    ? paramNames.slice(0, 3).join(', ') + (paramNames.length > 3 ? '...' : '')
    : 'no parameters';

  return (
    <div className="message-block function">
      <div className="function-header">
        <div className="message-type">
          <span className="tool-icon">🔧</span>
          <span>Function Call: <strong>{functionCall.name}</strong></span>
          <span className="response-meta">
            {paramCount} {paramCount === 1 ? 'parameter' : 'parameters'} • {formatBytes(inputSize)}
          </span>
        </div>
        <button 
          className="collapse-btn"
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "▶ Show" : "▼ Hide"}
        </button>
      </div>
      {collapsed && paramNames.length > 0 && (
        <div className="collapsed-preview">
          <code>{paramPreview}</code>
        </div>
      )}
      {!collapsed && (
        <div className="message-content">
          <div className="response-header">
            <span className="response-type-badge json-badge">Input Parameters</span>
            <span className="response-meta-info">
              {paramCount} {paramCount === 1 ? 'param' : 'params'} • {formatBytes(inputSize)}
            </span>
          </div>
          <div className="json-block-wrapper">
            <CopyButton content={inputStr} />
            <pre className="json-block">
              <code dangerouslySetInnerHTML={{ __html: highlightJSON(inputStr) }} />
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

function FunctionResponseBlock({ 
  functionResponse 
}: { 
  functionResponse: { name: string; response: string } 
}) {
  const [collapsed, setCollapsed] = useState(false);
  const parsed = parseToolResponse(functionResponse.response);
  const responseTypeIcon = getResponseTypeIcon(parsed.type);
  
  return (
    <div className="message-block function-response">
      <div className="function-header">
        <div className="message-type">
          <span className="tool-icon">{responseTypeIcon}</span>
          <span>Function Response: <strong>{functionResponse.name}</strong></span>
          <span className="response-meta">{getResponseMetadata(parsed)}</span>
        </div>
        <button 
          className="collapse-btn"
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "▶ Show" : "▼ Hide"}
        </button>
      </div>
      {!collapsed && (
        <div className="message-content">
          <ResponseContent parsed={parsed} />
        </div>
      )}
    </div>
  );
}

function ResponseContent({ parsed }: { parsed: ParsedToolResponse }) {
  const [showRaw, setShowRaw] = useState(false);

  if (parsed.type === "structured") {
    return (
      <div className="structured-response">
        {/* Metadata section */}
        {parsed.metadata && Object.keys(parsed.metadata).length > 0 && (
          <div className="response-metadata">
            {Object.entries(parsed.metadata).map(([key, value]) => (
              <div key={key} className="metadata-item">
                <span className="metadata-key">{formatMetadataKey(key)}:</span>
                <code className="metadata-value">{value}</code>
                {key === "filePath" && <CopyButton content={value} small />}
              </div>
            ))}
          </div>
        )}

        {/* Content section with collapse */}
        {parsed.content && (
          <CollapsibleCode 
            content={parsed.content} 
            language={detectLanguage(parsed.metadata?.filePath)}
          />
        )}

        {/* Toggle to show raw */}
        <button 
          className="show-raw-btn"
          onClick={() => setShowRaw(!showRaw)}
        >
          {showRaw ? "Hide Raw" : "Show Raw"}
        </button>

        {showRaw && (
          <div className="raw-response">
            <CopyButton content={parsed.raw} />
            <pre>{parsed.raw}</pre>
          </div>
        )}
      </div>
    );
  }

  if (parsed.type === "json") {
    const jsonObj = JSON.parse(parsed.raw);
    const isArray = Array.isArray(jsonObj);
    const isObject = jsonObj !== null && typeof jsonObj === "object" && !isArray;
    let label: string;
    let metaInfo: string;
    if (isArray) {
      label = "JSON Array";
      metaInfo = `${jsonObj.length} items`;
    } else if (isObject) {
      label = "JSON Object";
      metaInfo = `${Object.keys(jsonObj).length} keys`;
    } else {
      label = jsonObj === null ? "JSON null" : `JSON ${typeof jsonObj}`;
      metaInfo = String(jsonObj);
    }

    return (
      <div className="json-response">
        <div className="response-header">
          <span className="response-type-badge json-badge">
            {label}
          </span>
          <span className="response-meta-info">
            {metaInfo} • {formatBytes(new Blob([parsed.raw]).size)}
          </span>
        </div>
        <div className="json-block-wrapper">
          <CopyButton content={parsed.raw} />
          <pre className="json-block">
            <code dangerouslySetInnerHTML={{ __html: highlightJSON(parsed.raw) }} />
          </pre>
        </div>
      </div>
    );
  }

  // Plain text
  return (
    <div className="text-response">
      <CopyButton content={parsed.raw} />
      <div className="markdown">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>
          {parsed.raw}
        </ReactMarkdown>
      </div>
    </div>
  );
}

function CollapsibleCode({ 
  content, 
  language 
}: { 
  content: string; 
  language?: string 
}) {
  const lines = content.split('\n');
  const [expanded, setExpanded] = useState(lines.length <= 20);
  const displayLines = expanded ? lines : lines.slice(0, 20);

  return (
    <div className="collapsible-code">
      <div className="code-header">
        <span className="code-label">Content ({lines.length} lines)</span>
        <CopyButton content={content} small />
      </div>
      <pre className="code-block">
        <code>{displayLines.join('\n')}</code>
      </pre>
      {lines.length > 20 && (
        <button 
          className="expand-btn"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? `▲ Collapse (${lines.length - 20} lines hidden)` : `▼ Show all (${lines.length - 20} more lines)`}
        </button>
      )}
    </div>
  );
}

function CopyButton({ content, small = false }: { content: string; small?: boolean }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button 
      className={`copy-btn ${small ? 'small' : ''}`}
      onClick={handleCopy}
      title="Copy to clipboard"
    >
      {copied ? "✓" : "📋"}
    </button>
  );
}

// Utility functions

function parseToolResponse(response: string): ParsedToolResponse {
  // Try to parse as JSON first
  try {
    const parsed = JSON.parse(response);
    return {
      type: "json",
      raw: JSON.stringify(parsed, null, 2),
    };
  } catch {
    // Not JSON
  }

  // Check for structured XML-like response
  const xmlTagPattern = /<(\w+)>(.*?)<\/\1>/gs;
  const matches = Array.from(response.matchAll(xmlTagPattern));
  
  if (matches.length > 0) {
    const metadata: Record<string, string> = {};
    let content = "";

    for (const match of matches) {
      const [, tag, value] = match;
      if (tag === "content") {
        content = value.trim();
      } else {
        metadata[tag] = value.trim();
      }
    }

    return {
      type: "structured",
      metadata,
      content,
      raw: response,
    };
  }

  // Plain text
  return {
    type: "text",
    raw: response,
  };
}

function getResponseMetadata(parsed: ParsedToolResponse): string {
  if (parsed.type === "structured" && parsed.content) {
    const lines = parsed.content.split('\n').length;
    const size = new Blob([parsed.content]).size;
    return `${lines} lines, ${formatBytes(size)}`;
  }
  
  const size = new Blob([parsed.raw]).size;
  return formatBytes(size);
}

function getResponseTypeIcon(type: string): string {
  switch (type) {
    case "json":
      return "📊"; // JSON data
    case "structured":
      return "📄"; // Structured document
    case "text":
      return "📝"; // Plain text
    default:
      return "✓"; // Default checkmark
  }
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

function formatMetadataKey(key: string): string {
  // Convert camelCase or snake_case to Title Case
  return key
    .replace(/([A-Z])/g, ' $1')
    .replace(/_/g, ' ')
    .replace(/^./, str => str.toUpperCase())
    .trim();
}

function detectLanguage(filePath?: string): string | undefined {
  if (!filePath) return undefined;
  
  const ext = filePath.split('.').pop()?.toLowerCase();
  const langMap: Record<string, string> = {
    'js': 'javascript',
    'ts': 'typescript',
    'tsx': 'typescript',
    'jsx': 'javascript',
    'py': 'python',
    'go': 'go',
    'rs': 'rust',
    'java': 'java',
    'cpp': 'cpp',
    'c': 'c',
    'sh': 'bash',
    'yaml': 'yaml',
    'yml': 'yaml',
    'json': 'json',
    'html': 'html',
    'css': 'css',
    'md': 'markdown',
  };
  
  return ext ? langMap[ext] : undefined;
}

// Simple JSON syntax highlighting
function highlightJSON(json: string): string {
  return json
    .replace(/("([^"\\]|\\.)*")\s*:/g, '<span style="color: #9cdcfe">$1</span>:') // Keys
    .replace(/:\s*("([^"\\]|\\.)*")/g, ': <span style="color: #ce9178">$1</span>') // String values
    .replace(/:\s*(-?\d+\.?\d*)/g, ': <span style="color: #b5cea8">$1</span>') // Numbers
    .replace(/:\s*(true|false)/g, ': <span style="color: #569cd6">$1</span>') // Booleans
    .replace(/:\s*(null)/g, ': <span style="color: #569cd6">$1</span>'); // Null
}

// Mount the app
const root = createRoot(document.getElementById("root")!);
root.render(<App />);
