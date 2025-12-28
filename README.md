# Guardian

> **The Baseline Architecture for Your Local HIPAA Command Center**  
> Built by [Pocket Ninja LLC](https://pocketninja.co)

**Guardian** is Pocket Ninja's flagship demonstration of secure, offline-first HIPAA compliance tooling. It proves that sensitive healthcare data analysis doesn't require cloud uploads, third-party APIs, or compromised privacy.

This is the **reference implementation** we use to showcase our expertise in building local-first medical software that runs entirely on user hardware.

## Why We Built Guardian

Healthcare organizations need to verify HIPAA compliance, but they can't risk uploading Protected Health Information (PHI) to external services. Guardian solves this by:

- **Zero Cloud Dependency** - Your data never leaves your device
- **Offline AI** - Machine learning risk detection runs locally (no API keys, no telemetry)
- **Audit-Ready** - Generates compliance certificates with full history tracking
- **Open Architecture** - Built on Wails (Go + Vue.js), demonstrating modern desktop app patterns

**This is the "Baseline Architecture"** that Pocket Ninja LLC uses for all local HIPAA solutions. We're publishing Guardian to help the healthcare tech community build better, more secure tools.

---

## Architecture Overview

Guardian runs as a **single executable**. Inside, two parallel processes communicate via the Wails Bridge:

### 🧠 **The Brain (Go Backend)**
The "OS Operator" with root-level access to:
- **File System Service** - Native dialogs, batch processing, secure deletion
- **Local AI Engine** - Loads quantized LLMs, runs inference, scrubs PII
- **Risk Analyzer** - Pattern matching (SSN, MRN, dates) + ML classification
- **Certificate Generator** - PDF compliance reports with audit trails

### 🎨 **The Face (Vue.js Frontend)**
The "Command Dashboard" that visualizes data and captures user intent:
- **Drag & Drop Interface** - Pass file paths (not content) to Go for analysis
- **Real-time Progress** - Matrix-style terminal log showing backend operations
- **State Management (Pinia)** - Scan results, file queue, user preferences
- **Wails Bridge** - Seamless Go ↔ JavaScript communication

### Data Flow Example: "Scrub a Medical Record"
1. **User** drags a file onto the dropzone
2. **Vue** calls `AnalyzeFile(filePath)` (a Go function exposed to JS)
3. **Go** reads the file, runs regex + AI to find PHI, replaces it with `[REDACTED]`
4. **Vue** receives the cleaned text and displays original vs. scrubbed side-by-side

This architecture ensures **security** (no data leaks), **speed** (native Go performance), and **transparency** (users see exactly what's happening).

---

## Features

- 🔍 **Multi-Format Scanning** - PDF, DOCX, XLSX, TXT, images, and more
- 🧠 **Offline AI Classifier** - TF-IDF + N-gram detection (no cloud required)
- 📄 **Pattern Matching** - SSN, MRN, DOB, phone numbers, medical codes
- 🗓️ **Scheduled Audits** - Automatic recurring scans with history tracking
- 📊 **Compliance Certificates** - Official PDF reports for clean scans
- 🔒 **SQLite Storage** - All audit data stays local (Turso-ready for future sync)
- 📈 **Audit History** - Track compliance over time
- 💻 **Cross-Platform** - macOS (Universal) and Windows (x64)

---

## Installation

### macOS

1. Download `Guardian-macOS.zip` from [**Releases**](https://github.com/pocketninja-co/guardian/releases/latest)
2. Unzip and **right-click** `Guardian.app` → **Open**
3. Click "Open" on the Gatekeeper warning (first launch only)
4. (Optional) Drag to `/Applications` folder

### Windows

1. Download `Guardian-Windows.zip` from [**Releases**](https://github.com/pocketninja-co/guardian/releases/latest)
2. Unzip and run `Guardian.exe`
3. Click "More info" → "Run anyway" on SmartScreen warning

---

## Usage

### Quick File Scan
1. Launch Guardian
2. Click **"Add Files"** or drag files into the window
3. Review detected PHI risks with severity scores
4. Use **"Sanitize"** to create redacted copies

### Scheduled Compliance Audits
1. Go to **Settings** → **Schedule**
2. Add folders to monitor (e.g., `~/Documents`, `~/Desktop`)
3. Set scan frequency (daily, weekly, monthly)
4. Guardian runs audits automatically and generates certificates for clean scans

### Download Compliance Certificates
- After any clean scan, click **"Download Certificate"**
- Provides timestamped PDF proof of HIPAA compliance
- Includes full audit history for transparency

---

## For Developers

### Prerequisites
- Go 1.21+
- Node.js 18+
- Wails v2

### Build from Source

```bash
# Clone the repository
git clone https://github.com/pocketninja-co/guardian.git
cd guardian

# Install dependencies
go mod download
cd frontend && npm install && cd ..

# Run in development mode
wails dev

# Build production binaries
wails build -platform darwin/universal  # macOS
wails build -platform windows/amd64     # Windows
```

### Database Location
Guardian stores settings and audit logs in:
- **macOS**: `~/.hipaa_guardian/guardian.db`
- **Windows**: `%USERPROFILE%\.hipaa_guardian\guardian.db`

To reset:
```bash
# macOS/Linux
rm ~/.hipaa_guardian/guardian.db

# Windows (PowerShell)
Remove-Item $env:USERPROFILE\.hipaa_guardian\guardian.db
```

---

## About Pocket Ninja LLC

We build **local-first medical software** that prioritizes privacy, security, and user control. Guardian is our flagship open-source project demonstrating:

- **Offline AI** for sensitive data analysis
- **Zero-trust architecture** (data never leaves the device)
- **Modern desktop patterns** (Wails, Vue.js, Go)
- **HIPAA-ready design** from the ground up

**Need custom HIPAA software?** Guardian's architecture is the foundation for enterprise solutions. Contact us at [support@pocketninja.com](mailto:support@pocketninja.com).

---

## License

MIT License - see [LICENSE](LICENSE) file for details.

**Copyright © 2025 Pocket Ninja LLC**

---

## Support & Contributing

- **Issues**: [GitHub Issues](https://github.com/pocketninja-co/guardian/issues)
- **Discussions**: [GitHub Discussions](https://github.com/pocketninja-co/guardian/discussions)
- **Security**: Found a vulnerability? Email [support@pocketninja.com](mailto:support@pocketninja.com)

---

**⚠️ Disclaimer:** Guardian is a compliance **tool**, not a guarantee of compliance. It helps identify potential PHI but does not replace legal and compliance expertise. Always consult HIPAA professionals for your specific requirements.
