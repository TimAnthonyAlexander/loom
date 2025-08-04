# 🚀 **GUI Deployment Guide - Self-Contained Apps**

## 🎯 **Key Concept: No Web Server Needed!**

Wails creates **standalone executable files** that include:
- ✅ Your Go backend
- ✅ The React frontend (compiled)
- ✅ Built-in web server
- ✅ All dependencies

**Result**: One executable file that users can double-click to run! 🎉

---

## 📦 **Quick Deployment Commands**

### 1. **Build for Current Platform** ⭐ **MOST COMMON**
```bash
make build-gui
```
- Creates: `gui/build/gui` (macOS), `gui/build/gui.exe` (Windows)
- **Self-contained** - No additional files needed
- **Ready to distribute** - Just copy this one file!

### 2. **Build for All Platforms**
```bash
make deploy-gui-all
```
- Creates executables for: macOS (Intel + Apple Silicon), Windows, Linux
- Each platform gets its own folder
- **Perfect for releasing to users on different systems**

### 3. **Create Distribution Package**
```bash
make package-gui
```
- Builds the app and puts it in `dist/loom-gui`
- **Ready to zip and send** to users

### 4. **Build with Installer** (Windows)
```bash
make package-gui-installer
```
- Creates a Windows installer (.exe)
- **Professional distribution** for Windows users

---

## 🎯 **Deployment Options**

### **Option 1: Single File Distribution** ⭐ **EASIEST**

```bash
# Build for your platform
make build-gui

# The result is in gui/build/
# Just copy this file and send it to users!
cp gui/build/gui ~/Desktop/loom-gui
```

**How users run it:**
- **macOS/Linux**: Double-click or `./loom-gui`
- **Windows**: Double-click `loom-gui.exe`

### **Option 2: Cross-Platform Release**

```bash
# Build for all platforms
make deploy-gui-all

# Results in gui/build/:
# ├── bin/
# │   ├── loom-gui-darwin-amd64     (macOS Intel)
# │   ├── loom-gui-darwin-arm64     (macOS Apple Silicon)  
# │   ├── loom-gui-linux-amd64      (Linux)
# │   └── loom-gui-windows-amd64.exe (Windows)
```

**Create release packages:**
```bash
# Create zip files for each platform
cd gui/build/bin
zip loom-gui-macos-intel.zip loom-gui-darwin-amd64
zip loom-gui-macos-m1.zip loom-gui-darwin-arm64
zip loom-gui-linux.zip loom-gui-linux-amd64
zip loom-gui-windows.zip loom-gui-windows-amd64.exe
```

### **Option 3: Professional Installer** (Windows)

```bash
# Build with Windows installer
make package-gui-installer

# Creates: gui/build/loom-gui-installer.exe
# Users run installer, app appears in Start Menu
```

---

## 🏗️ **Custom Build Options**

### **Optimized Production Build**
```bash
cd gui
wails build -trimpath -upx -obfuscated
```

**Flags explained:**
- `-trimpath`: Remove file paths (smaller, more secure)
- `-upx`: Compress with UPX (smaller file size)
- `-obfuscated`: Code obfuscation (more secure)

### **Platform-Specific Builds**
```bash
# macOS only
wails build -platform darwin/arm64,darwin/amd64

# Windows only  
wails build -platform windows/amd64

# Linux only
wails build -platform linux/amd64
```

### **Debug vs Production**
```bash
# Development build (larger, with debug info)
wails build -debug

# Production build (optimized, smaller)
wails build
```

---

## 📤 **Distribution Strategies**

### **1. Direct File Sharing**
```bash
# Build and package
make package-gui

# Upload to cloud storage
# Send download link to users
```

### **2. GitHub Releases**
```bash
# Build all platforms
make deploy-gui-all

# Create GitHub release
# Attach platform-specific zip files
```

### **3. Website Download**
```bash
# Host on your website
# Provide download links for each platform
```

### **4. App Store Distribution**
For macOS App Store or Microsoft Store:
```bash
# Build without private APIs
wails build -platform darwin/arm64 -ldflags="-s -w"
# Then follow Apple/Microsoft signing process
```

---

## 🛡️ **Security & Signing**

### **macOS Code Signing**
```bash
# Sign the application (requires Apple Developer account)
codesign --force --deep --sign "Developer ID Application: Your Name" gui/build/gui

# Notarize for Gatekeeper
xcrun notarytool submit gui/build/gui --keychain-profile "notarytool-password"
```

### **Windows Code Signing**
```bash
# Sign with certificate (requires code signing certificate)
signtool sign /f certificate.p12 /p password gui/build/gui.exe
```

---

## 🎯 **User Experience**

### **What Users Get:**
1. **Single executable file** - No installation needed
2. **Native desktop app** - Feels like a real desktop application  
3. **Cross-platform** - Same experience on macOS, Windows, Linux
4. **No dependencies** - Works even without Go or Node.js installed
5. **Offline capable** - No internet required (unless your app needs it)

### **How It Works:**
1. User downloads your executable
2. User double-clicks or runs from terminal
3. **App opens instantly** - GUI appears as native window
4. **Everything included** - Web UI served from embedded files

---

## ✅ **Summary: Deployment Steps**

1. **Choose your deployment target:**
   - Single platform: `make build-gui`
   - All platforms: `make deploy-gui-all`
   - With installer: `make package-gui-installer`

2. **Test the executable:**
   ```bash
   ./gui/build/gui --help
   ```

3. **Distribute:**
   - Upload to file sharing service
   - Create GitHub release
   - Send directly to users

4. **Users run it:**
   - Double-click the file
   - OR run from command line
   - **GUI opens automatically!**

## 🎉 **That's It!**

Your Loom GUI is now a **professional, distributable desktop application** that users can run anywhere without any technical setup! 🚀