import { useState, useEffect } from 'react'
import Markdown from 'react-markdown'
import './App.css'

function App() {
  const [audits, setAudits] = useState([])
  const [selectedAudit, setSelectedAudit] = useState(null)
  const [auditContent, setAuditContent] = useState([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetch('http://localhost:3000/api/audits')
      .then(res => res.json())
      .then(data => setAudits(data))
      .catch(err => console.error("Failed to load audits", err))
  }, [])

  const loadAudit = (filename) => {
    setSelectedAudit(filename)
    setLoading(true)
    fetch(`http://localhost:3000/api/audits/${filename}`)
      .then(res => res.json())
      .then(data => {
        setAuditContent(data)
        setLoading(false)
      })
      .catch(err => {
        console.error("Failed to load audit content", err)
        setLoading(false)
      })
  }

  return (
    <div style={{ display: 'flex', height: '100vh', flexDirection: 'row' }}>
      <div style={{ width: '300px', borderRight: '1px solid #ccc', overflowY: 'auto', padding: '10px', boxSizing: 'border-box' }}>
        <h3>Audits</h3>
        <ul style={{ listStyle: 'none', padding: 0 }}>
          {audits.map(file => (
            <li 
              key={file} 
              onClick={() => loadAudit(file)}
              style={{ 
                cursor: 'pointer', 
                padding: '8px', 
                background: selectedAudit === file ? '#eee' : 'transparent',
                borderBottom: '1px solid #eee',
                wordBreak: 'break-all',
                fontSize: '12px'
              }}
            >
              {file}
            </li>
          ))}
        </ul>
      </div>
      <div style={{ flex: 1, padding: '20px', overflowY: 'auto', boxSizing: 'border-box' }}>
        {selectedAudit ? (
          <>
            <h2>{selectedAudit}</h2>
            {loading ? <p>Loading...</p> : (
              <div style={{ maxWidth: '800px', margin: '0 auto' }}>
                {auditContent.map((item, idx) => (
                  <div key={idx} style={{ marginBottom: '20px', padding: '15px', border: '1px solid #ddd', borderRadius: '5px', background: '#fff' }}>
                    {/* Render different types of messages */}
                    {item.system_prompt && (
                      <div>
                        <strong>System Prompt:</strong>
                        <div style={{ maxHeight: '200px', overflowY: 'auto', background: '#f5f5f5', padding: '10px', marginTop: '5px', fontSize: '12px' }}>
                          <Markdown>{item.system_prompt}</Markdown>
                        </div>
                      </div>
                    )}
                    
                    {item.type === 'initial_message' && (
                       <div>
                         <strong style={{ color: 'blue' }}>User:</strong>
                         <div style={{ marginTop: '5px' }}>
                            <Markdown>{item.content}</Markdown>
                         </div>
                       </div>
                    )}

                    {item.type === 'function_call' && (
                      <div>
                        <strong style={{ color: 'orange' }}>Function Call:</strong> {item.function_call?.name}
                        <pre style={{ background: '#333', color: '#fff', padding: '10px', overflowX: 'auto' }}>
                          {JSON.stringify(item.function_call?.input, null, 2)}
                        </pre>
                      </div>
                    )}

                    {item.type === 'function_result' && (
                      <div>
                        <strong style={{ color: 'green' }}>Function Result:</strong>
                        <pre style={{ background: '#f0fff0', padding: '10px', overflowX: 'auto', maxHeight: '300px' }}>
                          {item.content}
                        </pre>
                      </div>
                    )}

                    {item.type === 'assistant_message' && (
                      <div>
                         <strong style={{ color: 'purple' }}>Assistant:</strong>
                         <div style={{ marginTop: '5px' }}>
                            <Markdown>{item.content}</Markdown>
                         </div>
                      </div>
                    )}
                    
                    {/* Render unknown types nicely */}
                    {!item.type && !item.system_prompt && (
                        <pre>{JSON.stringify(item, null, 2)}</pre>
                    )}
                  </div>
                ))}
              </div>
            )}
          </>
        ) : (
          <p>Select an audit to view</p>
        )}
      </div>
    </div>
  )
}

export default App
