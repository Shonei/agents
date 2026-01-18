import sys
import os
import yaml
import markdown
from PyQt6.QtWidgets import (QApplication, QWidget, QVBoxLayout, QLineEdit,
                             QTextBrowser, QFrame, QGraphicsDropShadowEffect)
from PyQt6.QtCore import Qt, QThread, pyqtSignal
from PyQt6.QtGui import QColor, QKeyEvent
from google import genai
from google.genai import types

SYSTEM_PROMPT = """
You are a helpful assistant that helps the user quickly find information.

The user has the ability to type a quick question and you will answer it in a consise way.
The UI doesn't have a way to sohw long answers so be concise.
"""


# --- Configuration ---
# Read Gemini API Key from ~/.agents/config.yaml
def load_gemini_api_key():
    config_path = os.path.expanduser("~/.agents/config.yaml")
    try:
        with open(config_path, 'r') as f:
            config = yaml.safe_load(f)
            return config.get('gemini_api_key')
    except FileNotFoundError:
        return None
    except Exception as e:
        print(f"Error reading config file: {e}")
        return None

GEMINI_API_KEY = load_gemini_api_key()

class SearchThread(QThread):
    """
    Runs the Gemini query in a background thread.
    """
    results_ready = pyqtSignal(str)

    def __init__(self, query):
        super().__init__()
        self.query = query

    def run(self):
        if not GEMINI_API_KEY:
            self.results_ready.emit("Error: Gemini API key not found.\n\nPlease add 'gemini_api_key' to ~/.agents/config.yaml")
            return

        try:
            client = genai.Client(api_key=GEMINI_API_KEY)

            response = client.models.generate_content(
                model="gemini-3-flash-preview",
                  contents=self.query,
                  config = types.GenerateContentConfig(
                    temperature=0,
                    system_instruction=SYSTEM_PROMPT,
                )
            )
            
            # Extract text
            if response.text:
                self.results_ready.emit(response.text)
            else:
                self.results_ready.emit("No text returned from Gemini.")

        except Exception as e:
            self.results_ready.emit(f"Error connecting to Gemini:\n{str(e)}")

class Launcher(QWidget):
    def __init__(self):
        super().__init__()
        self.initUI()

    def initUI(self):
        # 1. Window Setup
        self.setWindowFlags(Qt.WindowType.FramelessWindowHint | Qt.WindowType.WindowStaysOnTopHint | Qt.WindowType.Tool)
        self.setAttribute(Qt.WidgetAttribute.WA_TranslucentBackground)
        self.resize(700, 400)
        self.center()

        # 2. Main Layout
        layout = QVBoxLayout()
        self.setLayout(layout)

        # 3. Styling Container (Rounded corners, background)
        self.container = QFrame()
        self.container.setStyleSheet("""
            QFrame {
                background-color: #2D2D2D;
                border-radius: 12px;
                border: 1px solid #3E3E3E;
            }
        """)
        
        # Add Drop Shadow
        shadow = QGraphicsDropShadowEffect()
        shadow.setBlurRadius(20)
        shadow.setColor(QColor(0, 0, 0, 100))
        shadow.setOffset(0, 5)
        self.container.setGraphicsEffect(shadow)

        container_layout = QVBoxLayout()
        self.container.setLayout(container_layout)
        layout.addWidget(self.container)

        # 4. Input Field
        self.input_field = QLineEdit()
        self.input_field.setPlaceholderText("Ask Gemini...")
        self.input_field.setStyleSheet("""
            QLineEdit {
                background-color: transparent;
                border: none;
                color: #FFFFFF;
                font-size: 24px;
                padding: 10px;
                selection-background-color: #555555;
            }
        """)
        self.input_field.returnPressed.connect(self.perform_search)
        container_layout.addWidget(self.input_field)

        # 5. Divider
        self.divider = QFrame()
        self.divider.setFrameShape(QFrame.Shape.HLine)
        self.divider.setStyleSheet("background-color: #3E3E3E;")
        container_layout.addWidget(self.divider)

        # 6. Results Area
        self.results_area = QTextBrowser()
        self.results_area.setReadOnly(True)
        self.results_area.setOpenExternalLinks(True)
        self.results_area.setStyleSheet("""
            QTextBrowser {
                background-color: transparent;
                border: none;
                color: #CCCCCC;
                font-size: 14px;
                padding: 10px;
            }
        """)
        container_layout.addWidget(self.results_area)

        # Focus input immediately
        self.input_field.setFocus()

    def center(self):
        screen = QApplication.primaryScreen().geometry()
        x = (screen.width() - self.width()) // 2
        y = (screen.height() - self.height()) // 4
        self.move(x, y)

    def keyPressEvent(self, event: QKeyEvent):
        if event.key() == Qt.Key.Key_Escape:
            QApplication.quit()

    def perform_search(self):
        query = self.input_field.text()
        if not query:
            return

        self.results_area.setText("Asking Gemini...")
        
        self.thread = SearchThread(query)
        self.thread.results_ready.connect(self.display_results)
        self.thread.start()

    def display_results(self, text):
        # Convert markdown to HTML with nice styling
        html_content = markdown.markdown(
            text,
            extensions=['fenced_code', 'codehilite', 'tables', 'nl2br']
        )

        # Wrap in styled HTML
        styled_html = f"""
        <html>
        <head>
            <style>
                body {{
                    color: #CCCCCC;
                    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
                    line-height: 1.6;
                    margin: 0;
                    padding: 0;
                }}
                h1, h2, h3, h4, h5, h6 {{
                    color: #FFFFFF;
                    margin-top: 1em;
                    margin-bottom: 0.5em;
                }}
                h1 {{ font-size: 1.8em; border-bottom: 2px solid #3E3E3E; padding-bottom: 0.3em; }}
                h2 {{ font-size: 1.5em; border-bottom: 1px solid #3E3E3E; padding-bottom: 0.3em; }}
                h3 {{ font-size: 1.3em; }}
                code {{
                    background-color: #1E1E1E;
                    color: #D4D4D4;
                    padding: 2px 6px;
                    border-radius: 3px;
                    font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
                    font-size: 0.9em;
                }}
                pre {{
                    background-color: #1E1E1E;
                    border: 1px solid #3E3E3E;
                    border-radius: 6px;
                    padding: 12px;
                    overflow-x: auto;
                    margin: 1em 0;
                }}
                pre code {{
                    background-color: transparent;
                    padding: 0;
                    color: #D4D4D4;
                }}
                blockquote {{
                    border-left: 4px solid #3E3E3E;
                    margin: 1em 0;
                    padding-left: 1em;
                    color: #999999;
                }}
                a {{
                    color: #4A9EFF;
                    text-decoration: none;
                }}
                a:hover {{
                    text-decoration: underline;
                }}
                ul, ol {{
                    margin: 0.5em 0;
                    padding-left: 2em;
                }}
                li {{
                    margin: 0.3em 0;
                }}
                table {{
                    border-collapse: collapse;
                    width: 100%;
                    margin: 1em 0;
                }}
                th, td {{
                    border: 1px solid #3E3E3E;
                    padding: 8px 12px;
                    text-align: left;
                }}
                th {{
                    background-color: #1E1E1E;
                    color: #FFFFFF;
                    font-weight: bold;
                }}
                tr:nth-child(even) {{
                    background-color: #252525;
                }}
                hr {{
                    border: none;
                    border-top: 1px solid #3E3E3E;
                    margin: 1.5em 0;
                }}
                strong {{
                    color: #FFFFFF;
                }}
                em {{
                    color: #E0E0E0;
                }}
            </style>
        </head>
        <body>
            {html_content}
        </body>
        </html>
        """

        self.results_area.setHtml(styled_html)

    def focusOutEvent(self, event):
        super().focusOutEvent(event)

if __name__ == '__main__':
    app = QApplication(sys.argv)
    ex = Launcher()
    ex.show()
    ex.activateWindow()
    ex.raise_()
    ex.input_field.setFocus()
    sys.exit(app.exec())
