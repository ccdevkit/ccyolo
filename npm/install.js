#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const zlib = require('zlib');
const https = require('https');
const { execSync } = require('child_process');

const PACKAGE_VERSION = require('./package.json').version;

// Map Node.js platform/arch to Go GOOS/GOARCH
const PLATFORM_MAP = {
  'darwin-x64': { goos: 'darwin', goarch: 'amd64' },
  'darwin-arm64': { goos: 'darwin', goarch: 'arm64' },
  'linux-x64': { goos: 'linux', goarch: 'amd64' },
  'linux-arm64': { goos: 'linux', goarch: 'arm64' },
  'win32-x64': { goos: 'windows', goarch: 'amd64' },
  'win32-arm64': { goos: 'windows', goarch: 'arm64' },
};

const platformKey = `${process.platform}-${process.arch}`;
const platformInfo = PLATFORM_MAP[platformKey];

if (!platformInfo) {
  console.error(`Unsupported platform: ${platformKey}`);
  console.error(`Supported platforms: ${Object.keys(PLATFORM_MAP).join(', ')}`);
  process.exit(1);
}

const { goos, goarch } = platformInfo;
const binaryName = process.platform === 'win32' ? 'ccyolo.exe' : 'ccyolo';
const assetName = `ccyolo-${goos}-${goarch}.tar.gz`;
const downloadUrl = `https://github.com/ccdevkit/ccyolo/releases/download/v${PACKAGE_VERSION}/${assetName}`;
const binDir = path.join(__dirname, 'bin');
const binaryPath = path.join(binDir, binaryName);

function makeRequest(url) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, (response) => {
      // Handle redirects (GitHub releases redirect to S3)
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        makeRequest(response.headers.location).then(resolve, reject);
        return;
      }

      if (response.statusCode < 200 || response.statusCode >= 300) {
        reject(new Error(`Failed to download: HTTP ${response.statusCode}`));
        return;
      }

      const chunks = [];
      response.on('data', (chunk) => chunks.push(chunk));
      response.on('end', () => resolve(Buffer.concat(chunks)));
      response.on('error', reject);
    });

    request.on('error', reject);
  });
}

function extractTarGz(buffer, destDir) {
  // Use tar command for extraction
  // Windows 10+ includes tar.exe, and Git Bash/WSL also provide it
  const tarball = path.join(destDir, 'temp.tar.gz');
  fs.writeFileSync(tarball, buffer);

  try {
    // Use tar command - available on Windows 10+, macOS, and Linux
    execSync(`tar -xzf "${tarball}" -C "${destDir}"`, { stdio: 'pipe' });
  } finally {
    fs.unlinkSync(tarball);
  }
}

async function install() {
  console.log(`Downloading ccyolo for ${goos}/${goarch}...`);
  console.log(`URL: ${downloadUrl}`);

  try {
    // Ensure bin directory exists
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    const buffer = await makeRequest(downloadUrl);
    console.log(`Downloaded ${buffer.length} bytes`);

    extractTarGz(buffer, binDir);

    // Make binary executable (not needed on Windows)
    if (process.platform !== 'win32') {
      fs.chmodSync(binaryPath, 0o755);
    }

    console.log(`Installed ccyolo to ${binaryPath}`);
  } catch (error) {
    console.error(`Failed to install ccyolo: ${error.message}`);
    console.error('');
    console.error('You can manually download the binary from:');
    console.error(`https://github.com/ccdevkit/ccyolo/releases/tag/v${PACKAGE_VERSION}`);
    process.exit(1);
  }
}

install();
