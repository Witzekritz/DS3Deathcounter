# DSDeathcounter

A death counter for FromSoftware games that reads the death count directly from memory and displays it through a local web interface with game-specific styling.

## Supported Games

- Dark Souls
- Dark Souls II
- Dark Souls III
- Dark Souls Remastered
- Sekiro: Shadows Die Twice

## Features

- Automatically detects running games
- Updates death count in real-time
- Game-specific visual themes
- Seamlessly switches between games without requiring restart
- Designed for easy integration with streaming software

## Installation

- Download the latest executable from the [Releases](https://github.com/Witzekritz/DSDeathcounter/releases) page

## Build from Source

1. Install Go (version 1.20 or higher recommended)
2. Clone the repository:
   ```
   git clone https://github.com/Witzekritz/DSDeathcounter.git
   ```
3. Build the executable:
   ```
   cd DSDeathcounter
   go build -o dsdeathcounter.exe ./cmd/dsdeathcounter
   ```

## Usage

1. Start any supported FromSoftware game
2. Run the dsdeathcounter executable
3. Open http://localhost:8080 in your browser or add http://localhost:8080 as a Browser Source in OBS Studio

The counter will automatically detect which game is running and display the appropriate death counter with game-specific styling. If you switch games, the counter will automatically update without requiring a restart.

## OBS Studio Integration

1. Add a new Browser source to your scene
2. Set the URL to http://localhost:8080
3. Set width and height as needed (recommended: 300x100)

## License

This project is licensed under the MIT License - see the LICENSE file for details.
