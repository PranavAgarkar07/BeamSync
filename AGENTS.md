# Agent Pre-Push Protocol

Before pushing any changes to the repository, execute every step below in order. Do not skip steps.

---

## 1. Pull latest `main` and rebase

```bash
git fetch origin
git rebase origin/main
```

If conflicts arise, resolve them, then `git rebase --continue`.

---

## 2. Read the version file

The canonical version lives in `desktop/VERSION`. Always read it before building:

```bash
VERSION=$(tr -d '[:space:]' < desktop/VERSION)
```

If you changed the version, update these files to match:

| File | What to change |
|---|---|
| `desktop/VERSION` | Write the new version (e.g. `2.5.0`) |
| `desktop/wails.json` | `info.productVersion` |
| `.aur/beamsync-bin/PKGBUILD` | `pkgver` and `sha256sums` |
| `desktop/packaging/arch/PKGBUILD` | `pkgver` and `sha256sums` |
| `PKGBUILD_aur_temp` | `pkgver` and `sha256sums` |
| `.aur/beamsync-bin/.SRCINFO` | `pkgver`, source URL, and `sha256sums` |

After changing `desktop/VERSION`, rebuild the binary (see step 4) so the embedded version is correct.

---

## 3. Run all available checks

### Go

```bash
cd desktop
go vet ./...
cd ../beamsync
go vet ./...
cd ..
```

If either fails, fix all issues before proceeding.

### Frontend

```bash
cd desktop/frontend
npm run build
cd ../..
```

Frontend must build without errors.

---

## 4. Build the binary

```bash
cd desktop && wails build -tags webkit2_41 -o BeamSync && cd ..
```

Verify the binary has the correct embedded version (raw VERSION + "v" prefixed):

```bash
strings desktop/build/bin/BeamSync | grep -E '(^|[^0-9])v?[0-9]+\.[0-9]+\.[0-9]+([^0-9]|$)' | head -5
```

The output must match the version in `desktop/VERSION`.

---

## 5. Update packaging binaries and checksums

If the binary changed, copy it and recompute checksums:

```bash
cp desktop/build/bin/BeamSync desktop/packaging/arch/BeamSync
sha256sum desktop/packaging/arch/BeamSync
```

Update `sha256sums` in all three PKGBUILD files and `.SRCINFO` with the new hash.

---

## 6. Commit checklist

Before writing the commit message, confirm:

- [ ] `go vet ./...` passes in both `desktop/` and `beamsync/`
- [ ] Frontend `npm run build` succeeds
- [ ] Binary builds with `wails build -tags webkit2_41`
- [ ] Binary embeds the correct version (`strings | grep` check)
- [ ] All PKGBUILD checksums are updated
- [ ] `.SRCINFO` is in sync with PKGBUILD
- [ ] No debug code, commented-out blocks, or `fmt.Println` leftovers (except in `app.go`'s update checker)
- [ ] `desktop/wails.json` productVersion matches `desktop/VERSION`
- [ ] Commit message follows conventional commits format

### Commit message format

```
<type>: <brief description>

<optional body>
```

Types: `feat` `fix` `chore` `docs` `refactor` `test` `style`

Examples:
```
feat: add transfer pause/resume
fix: crash when scanning QR code on mobile
docs: update install instructions for Arch
```

---

## 7. Push

```bash
git push origin <branch>
```

If push fails due to hooks, read the hook output, fix the issue, amend the commit, and push again:

```bash
# after fixing
git add .
git commit --amend --no-edit
git push origin <branch> --force-with-lease
```

---

## 8. Open a Pull Request

Title must match the commit message. Body must include:

```markdown
## Summary

<what changed and why>

## Verification

- [ ] `go vet` passes
- [ ] Frontend builds
- [ ] Binary version matches VERSION file
- [ ] PKGBUILD checksums updated
```

---

---

## 9. Create a Release

Only the maintainer runs this. It tags the commit, builds cross-platform binaries, creates a GitHub release, and publishes the AUR package.

### 9a. Tag the release

```bash
VERSION=$(tr -d '[:space:]' < desktop/VERSION)
git tag -a "v$VERSION" -m "Release v$VERSION"
git push origin "v$VERSION"
```

### 9b. Build cross-platform binaries

Wails can cross-compile. Run each on the appropriate host OS or use a CI matrix.

> **NOTE:** macOS binaries are cross-compiled and **untested** — only Linux and Windows machines are available for testing.

**Linux (amd64) — tested:**
```bash
cd desktop && wails build -tags webkit2_41 -o BeamSync -platform linux/amd64 && cd ..
cp desktop/build/bin/BeamSync "BeamSync-v$VERSION-linux-amd64"
```

**Linux (arm64) — untested:**
```bash
cd desktop && wails build -tags webkit2_41 -o BeamSync -platform linux/arm64 && cd ..
cp desktop/build/bin/BeamSync "BeamSync-v$VERSION-linux-arm64"
```

**macOS (amd64) — untested:**
```bash
cd desktop && wails build -o BeamSync -platform darwin/amd64 && cd ..
cp desktop/build/bin/BeamSync "BeamSync-v$VERSION-darwin-amd64"
```

**macOS (arm64) — untested:**
```bash
cd desktop && wails build -o BeamSync -platform darwin/arm64 && cd ..
cp desktop/build/bin/BeamSync "BeamSync-v$VERSION-darwin-arm64"
```

**Windows (amd64) — tested:**
```bash
cd desktop && wails build -o BeamSync.exe -platform windows/amd64 && cd ..
cp desktop/build/bin/BeamSync.exe "BeamSync-v$VERSION-windows-amd64.exe"
```

### 9c. Compute checksums for all artifacts

```bash
sha256sum BeamSync-v$VERSION-* > SHA256SUMS.txt
```

### 9d. Create GitHub release

```bash
gh release create "v$VERSION" \
  --title "v$VERSION" \
  --notes "$(cat release_notes.md)" \
  BeamSync-v$VERSION-linux-amd64 \
  BeamSync-v$VERSION-linux-arm64 \
  BeamSync-v$VERSION-darwin-amd64 \
  BeamSync-v$VERSION-darwin-arm64 \
  BeamSync-v$VERSION-windows-amd64.exe \
  SHA256SUMS.txt
```

### 9e. Publish AUR package

The AUR lives in `.aur/beamsync-bin/`. It is a separate git repo that tracks the AUR `master` branch.

```bash
# Ensure .aur/beamsync-bin is a clean checkout of ssh://aur@aur.archlinux.org/beamsync-bin.git
# (or do this once: git -C .aur/beamsync-bin remote add aur ssh://aur@aur.archlinux.org/beamsync-bin.git)

cd .aur/beamsync-bin

# Regenerate .SRCINFO from PKGBUILD
makepkg --printsrcinfo > .SRCINFO

# Commit and push to AUR
git add PKGBUILD .SRCINFO beamsync.desktop
git commit -m "Update to v$VERSION"
git push aur master
cd ../..
```

---

## Quick reference

### Pre-push pipeline
```bash
cd desktop && go vet ./... && cd ../beamsync && go vet ./... && cd ..
cd desktop/frontend && npm run build && cd ../..
cd desktop && wails build -tags webkit2_41 -o BeamSync && cd ..
strings desktop/build/bin/BeamSync | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$'
cp desktop/build/bin/BeamSync desktop/packaging/arch/BeamSync
sha256sum desktop/packaging/arch/BeamSync
```

### Release pipeline
```bash
VERSION=$(tr -d '[:space:]' < desktop/VERSION)
git tag -a "v$VERSION" -m "Release v$VERSION"
git push origin "v$VERSION"
# build for each platform (see §9b)
sha256sum BeamSync-v$VERSION-* > SHA256SUMS.txt
gh release create "v$VERSION" --title "v$VERSION" --notes "$(cat release_notes.md)" BeamSync-v$VERSION-* SHA256SUMS.txt
cd .aur/beamsync-bin && makepkg --printsrcinfo > .SRCINFO && git add -A && git commit -m "Update to v$VERSION" && git push aur master
```
