# Guardian 🛡️

![Status](https://img.shields.io/badge/Status-Beta-blue)
![Privacy](https://img.shields.io/badge/Privacy-Local--Only-green)
![Build](https://img.shields.io/badge/Build-Wails%20v2-red)
![License](https://img.shields.io/badge/License-MIT-lightgrey)

**The "Safe Harbor" utility for modern medical practices.**
> **Built by [Pocket Ninja LLC](https://pocketninja.co)**

Guardian is a local desktop application that allows healthcare professionals to securely "scrub" sensitive patient data (PHI) from files, enabling the safe use of modern AI tools like ChatGPT without violating privacy laws.

> **🔒 Security Promise:** Guardian follows a zero-trust, local-first architecture. No data—encrypted or otherwise—is ever sent to the cloud. All processing happens 100% offline on your machine.

---

## 🚀 Why Use Guardian?

Medical data is often "dirty" and trapped in legacy formats. Practice managers want to use AI to draft appeal letters or analyze billing trends, but they are blocked by HIPAA compliance risks.

**Guardian bridges the gap.**

### For Practice Managers 👩‍⚕️
* **The "ChatGPT" Blocker:** You want to use AI to write a denial appeal letter, but you can't paste patient info into a public chatbot.
* **The Fix:** Drag your file into Guardian. It gives you a "Scrubbed" version in seconds. You can now safely paste that text into ChatGPT.

### For IT Directors & MSPs 💻
* **The "Desktop" Liability:** Staff members often save unencrypted CSVs of patient dumps to their Desktop or Downloads folder, creating massive liability.
* **The Fix:** Guardian monitors local folders for "ticking time bombs" (unencrypted PHI) and alerts the user *before* a breach happens.

---

## ✨ Core Capabilities

* **🔍 Smart De-identification:** Automatically finds and redacts 18 HIPAA identifiers (Names, SSNs, MRNs, Dates) using a hybrid engine (Regex + Local Machine Learning).
* **🛡️ Active Sentinel:** Runs quietly in the background.
    * **Green Shield:** System Secure.
    * **Red Shield:** Risk Detected (e.g., `patient_dump.csv` found in Downloads).
* **📄 Compliance Certificates:** Generates a cryptographically signed PDF certificate for every clean scan, creating a verifiable audit trail for your internal records.
* **🗓️ Scheduled Audits:** Set it and forget it. Guardian automatically scans high-risk folders (Downloads, Desktop) on a daily or weekly schedule.
* **🔒 Local Vault:** All audit history is stored in an encrypted local SQLite database (Turso-ready).

---

## 🛠️ Technical Architecture

Guardian is the **reference implementation** of Pocket Ninja's offline-first security stack. It runs as a single executable containing two parallel processes:

### 🧠 **The Brain (Go Backend)**
The "OS Operator" with root-level access. It handles the heavy lifting without touching the internet:
* **Local AI Engine:** Loads quantized models to run inference and scrub PII locally.
* **Risk Analyzer:** Performs high-speed pattern matching (SSN, MRN) and ML classification.
* **File System Service:** Handles native dialogs, batch processing, and secure deletion.

### 🎨 **The Face (Vue.js Frontend)**
The "Command Dashboard" for user interaction:
* **Drag & Drop Interface:** Passes file paths (never content) to the backend.
* **Real-time Logic:** Visualizes the "Matrix-style" scanning process in real-time.
* **State Management:** Powered by Pinia for fast, reactive updates.

---

## 📥 Installation

### macOS
1.  Download `Guardian-macOS.zip` from [**Releases**](https://github.com/pocketninja-co/guardian/releases/latest).
2.  Unzip and **right-click** `Guardian.app` → **Open**.
3.  Click "Open" on the Gatekeeper warning (required for the first launch only).

### Windows
1.  Download `Guardian-Windows.zip` from [**Releases**](https://github.com/pocketninja-co/guardian/releases/latest).
2.  Unzip and run `Guardian.exe`.
3.  Click "More info" → "Run anyway" if prompted by SmartScreen.

---

## ⚡ How to Use

### 1. Scrubbing a File (The "AI-Ready" Workflow)
1.  Launch Guardian.
2.  Drag a clinical note or CSV onto the **"Dropzone"**.
3.  Review the **Risk Score** (e.g., "Critical - 12 SSNs found").
4.  Click **"Sanitize"**.
5.  **Result:** A clean file is saved to your desktop, ready for use in AI or analytics.

### 2. Downloading Proof
After any successful scan, click **"Download Certificate"**. This provides a timestamped PDF proving that you performed due diligence on the data.

---

## For Developers

Guardian is built on the **Wails v2** framework (Go + Vue.js).

### Prerequisites
* Go 1.21+
* Node.js 18+
* Wails v2 CLI

### Build Commands

```bash
# Clone the repository
git clone [https://github.com/pocketninja-co/guardian.git](https://github.com/pocketninja-co/guardian.git)
cd guardian

# Install dependencies
go mod download
cd frontend && npm install && cd ..

# Run in development mode (Hot Reload)
wails dev

# Build production binaries
wails build -platform darwin/universal  # macOS
wails build -platform windows/amd64     # Windows
