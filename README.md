# Guardian

> A local HIPAA compliance tool for macOS & Windows by [Pocket Ninja LLC](https://pocketninja.co)

**Guardian** helps you detect and remediate Protected Health Information (PHI) in files on your local machine, ensuring HIPAA compliance without sending data to the cloud.

## Features

- 🔍 **Local File Scanning** - Analyze documents for PHI patterns (SSN, MRN, dates of birth, etc.)
- 🧠 **Offline AI Classifier** - Machine learning-based risk detection (no cloud required)
- 📄 **Multi-Format Support** - PDF, DOCX, XLSX, TXT, images, and more
- 🗓️ **Scheduled Audits** - Automatic recurring scans of specified directories
- 📊 **Compliance Certificates** - Generate official PDF certificates for clean scans
- 🔒 **Local Storage** - All data stays on your device (SQLite database)
- 📈 **Audit History** - Track scan results over time
- 💻 **Cross-Platform** - Available for macOS and Windows

## Installation

### macOS

1. Go to the [**Releases**](https://github.com/pocketninja-co/guardian/releases/latest) page
2. Download `Guardian-macOS.zip`
3. Unzip the file

#### First Launch (macOS Only - Important!)

Since Guardian is currently unsigned, macOS Gatekeeper will block it. To open:

1. **Right-click** (or Control+Click) on `Guardian.app`
2. Select **"Open"** from the menu
3. Click **"Open"** again in the dialog that appears

After this first launch, you can open Guardian normally.

#### Drag to Applications (macOS - Optional)

For easier access, drag `Guardian.app` to your `/Applications` folder.

### Windows

1. Go to the [**Releases**](https://github.com/pocketninja-co/guardian/releases/latest) page
2. Download `Guardian-Windows.zip`
3. Unzip the file
4. Run `Guardian.exe`

**Note:** Windows may show a SmartScreen warning since the app is unsigned. Click "More info" → "Run anyway"

## Usage

### Quick Scan
1. Launch Guardian
2. Click **"Add Files"** or drag files into the window
3. Review detected risks
4. Use **"Sanitize"** to clean risky files

### Scheduled Audits
1. Go to **Settings** tab
2. Enable **Schedule**
3. Choose folders to monitor
4. Set scan frequency
5. Guardian will run audits automatically and generate certificates

### Download Certificates
- After a clean scan, click **"Download Certificate"** to save a PDF report
- Certificates include audit history and compliance proof

## Development

### Prerequisites
- Go 1.21+
- Node.js 18+
- Wails v2

### Build from Source

```bash
# Clone
git clone https://github.com/pocketninja-co/guardian.git
cd guardian

# Install dependencies
go mod download
cd frontend && npm install && cd ..

# Run in dev mode
wails dev

# Build production
wails build -platform darwin/universal
```

### Database Location
Guardian stores settings and audit logs in: `~/.hipaa_guardian/guardian.db`

To reset:
```bash
rm ~/.hipaa_guardian/guardian.db
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

**Copyright © 2025 Pocket Ninja LLC**

## Support

For issues or feature requests, please open an issue on [GitHub](https://github.com/pocketninja-co/guardian/issues).

---

**⚠️ Disclaimer:** Guardian is a tool to help identify potential PHI. It does not guarantee 100% detection or compliance. Always consult with legal and compliance professionals for your specific HIPAA requirements.
