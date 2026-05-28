#!/bin/bash
# BeamSync — auto-configure firewall for ports 3000-3100
# Self-elevating (pkexec → pinentry askpass → bare sudo)
PORTS="3000:3100"

if [ "$EUID" -ne 0 ]; then
  if command -v pkexec >/dev/null; then
    pkexec "$0" && exit 0 || true
  fi

  if command -v sudo >/dev/null; then
    for p in pinentry pinentry-gnome3 pinentry-qt pinentry-qt5; do
      ap=$(command -v "$p" 2>/dev/null || true)
      [ -n "$ap" ] && break
    done
    if [ -n "$ap" ]; then
      askpass=$(mktemp)
      cat > "$askpass" << 'ASKEOF'
#!/bin/bash
printf 'SETDESC BeamSync Firewall Setup\nSETPROMPT Password:\nGETPIN\nBYE\n' | "$PINENTRY" --display="${DISPLAY:-:0}" 2>/dev/null | sed -n 's/^D //p'
ASKEOF
      chmod +x "$askpass"
      export PINENTRY="$ap" SUDO_ASKPASS="$askpass"
      sudo -A "$0"; rc=$?
      rm -f "$askpass"
      exit $rc
    fi

    sudo "$0"; exit $?
  fi

  echo "❌ Cannot elevate. Run: sudo $0"
  exit 1
fi

echo "🛡️ Configuring Firewall for BeamSync (ports $PORTS)..."
if command -v ufw >/dev/null; then
  ufw allow "$PORTS/tcp" comment 'BeamSync'
  echo "✅ UFW Rules Added for ports $PORTS (TCP)."
  ufw status | grep 3000
elif command -v firewall-cmd >/dev/null; then
  firewall-cmd --permanent --add-port="$PORTS/tcp"
  firewall-cmd --reload
  echo "✅ Firewalld Rules Added for ports $PORTS (TCP)."
elif command -v iptables >/dev/null; then
  iptables -A INPUT -p tcp --match multiport --dports "$PORTS" -j ACCEPT
  echo "✅ iptables rule added for ports $PORTS (TCP)."
else
  echo "❌ No supported firewall found (ufw, firewalld, iptables)."
  exit 1
fi
