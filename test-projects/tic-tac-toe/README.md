# Tic-Tac-Toe Game

A modern, fully-featured Tic-Tac-Toe game built with Bun, React, and Vite.

## Features

- **Interactive gameplay**: Click on squares to place your mark (X or O)
- **Win detection**: Automatically detects when a player wins and highlights the winning line
- **Draw detection**: Recognizes when the game ends in a draw
- **Move history**: Track all moves made during the game
- **Time travel**: Jump back to any previous move to see the board state
- **Reset game**: Start a fresh game at any time
- **Responsive design**: Works seamlessly on desktop and mobile devices
- **Modern UI**: Beautiful gradient design with smooth animations

## Tech Stack

- **Bun**: Fast JavaScript runtime and package manager
- **React 19**: Latest React with hooks
- **Vite**: Lightning-fast build tool
- **CSS3**: Custom styling with animations

## Getting Started

### Prerequisites

- Bun installed on your system (https://bun.sh)

### Installation

```bash
cd tic-tac-toe
bun install
```

### Development

```bash
bun dev
```

Open your browser and navigate to `http://localhost:5173`

### Build for Production

```bash
bun run build
```

### Preview Production Build

```bash
bun run preview
```

## How to Play

1. The game starts with player X
2. Click on any empty square to place your mark
3. Players alternate turns (X and O)
4. First player to get three marks in a row (horizontally, vertically, or diagonally) wins
5. If all squares are filled with no winner, the game is a draw
6. Use the "Move History" panel to jump back to any previous state
7. Click "Reset Game" to start over

## Project Structure

```
tic-tac-toe/
├── src/
│   ├── App.jsx        # Main game component with logic
│   ├── App.css        # Game styling
│   ├── main.jsx       # React entry point
│   └── index.css      # Global styles
├── public/            # Static assets
├── package.json       # Dependencies and scripts
└── vite.config.js     # Vite configuration
```

## Game Logic

The game implements:

- **State management**: Using React hooks (useState)
- **Immutability**: Creating new arrays instead of mutating state
- **Winner calculation**: Checking all possible winning combinations
- **History tracking**: Storing all game states for time travel
- **Turn management**: Alternating between X and O based on move count

## License

MIT
