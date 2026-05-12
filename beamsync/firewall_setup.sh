#!/bin/bash

# BeamSync Helper Script
# Configures Linux Firewall to allow ports 3000-3100 for BeamSync.
# Requires sudo/root privileges.

if [ "$EUID" -ne 0 ]; then
  echo "❌ Please run as root (sudo)."
  exit 1
fi

echo "🚀 Configuring Firewall for BeamSync..."

# Detect UFW (Ubuntu/Debian/Mint)
if command -v ufw >/dev/null; then
    echo "🔧 Detected UFW. Adding rules..."
    ufw allow 3000:3100/tcp comment 'BeamSync'
    echo "✅ UFW Rules Added for ports 3000-3100 (TCP)."
    ufw status | grep 3000
    exit 0
fi

# Detect Firewalld (Fedora/RHEL/CentOS)
if command -v firewall-cmd >/dev/null; then
    echo "🔧 Detected Firewalld. Adding rules..."
    firewall-cmd --permanent --add-port=3000-3100/tcp
    firewall-cmd --reload
    echo "✅ Firewalld Rules Added for ports 3000-3100 (TCP)."
    exit 0
fi

# Detect IPtables (Generic) - Last resort, simpler to just warn user
if command -v iptables >/dev/null; then
    echo "⚠️  Detected raw iptables. Configuring raw iptables rules is complex and risky via script."
    echo "ℹ️  Please manually allow TCP traffic on ports 3000 to 3100."
    echo "   Example: iptables -A INPUT -p tcp --match multiport --dports 3000:3100 -j ACCEPT"
    exit 1
fi

echo "❌ No supported firewall manager found (ufw, firewalld)."
echo "ℹ️  Please manually allow TCP ports 3000-3100."
