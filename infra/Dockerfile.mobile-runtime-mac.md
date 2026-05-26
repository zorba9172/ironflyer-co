# Ironflyer iOS-build host provisioning

macOS cannot be containerised — Apple's licence and the Darwin kernel
both forbid it. The Ironflyer iOS Pro tier therefore runs `xcodebuild`
on a bare-metal (or Tart-virtualised) Mac mini that the Linux runtime
dispatches work to over SSH.

This file documents the one-time provisioning steps for that host. It
is intentionally a Markdown checklist rather than a Dockerfile: there
is no image to build, only a recipe to apply.

## Hardware

- Mac mini M2 or M4 (16 GB RAM minimum; 24 GB recommended when
  building multiple flavours in parallel).
- Scaleway Apple Silicon and MacStadium are the supported managed
  options. Self-hosted minis work but the operator owns physical
  recovery.

## Account

- Create a dedicated build account `ironflyer-build` with admin rights
  (required for the first `xcodebuild` accept-licence dialog and for
  `security` keychain operations).
- Enable auto-login for `ironflyer-build` so the host comes up
  unattended after power events. In **System Settings → Users &
  Groups → Login Options**, set the auto-login user to
  `ironflyer-build`.

## Xcode

```bash
xcode-select --install            # CLT for Homebrew prereqs
sudo softwareupdate --install-rosetta --agree-to-license   # Apple Silicon only
```

Install Xcode from the App Store at the version pinned by the
orchestrator's mobile blueprint (current: Xcode 16.x). Then accept the
licence and prefetch the simulator runtimes:

```bash
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
sudo xcodebuild -license accept
xcodebuild -downloadAllPlatforms                # downloads iOS simulator runtimes
```

## Homebrew + tooling

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
brew install xcodegen fastlane node@22 swiftlint coreutils gnu-sed
brew install --cask tart           # iOS simulator VM management
```

`xcodegen` is mandatory: the Ironflyer starter template ships
`project.yml` only, and the build driver runs `make generate` (which in
turn invokes xcodegen) before any `xcodebuild` call.

## Runtime SSH wiring

The Linux runtime needs to dispatch work to this host. We use SSH key
auth, not password auth.

```bash
sudo systemsetup -setremotelogin on
sudo dseditgroup -o edit -a ironflyer-build -t user com.apple.access_ssh

# On the runtime host, generate or reuse the runtime's SSH key, then
# append the public key to the Mac's authorised_keys:
mkdir -p ~/.ssh && chmod 700 ~/.ssh
cat >> ~/.ssh/authorized_keys <<'PUB'
ssh-ed25519 AAAA... runtime@ironflyer
PUB
chmod 600 ~/.ssh/authorized_keys
```

## Environment

Set in `~/.zprofile` for the `ironflyer-build` user so unattended ssh
sessions inherit them:

```bash
export IRONFLYER_MAC_POOL_ENABLED=1
export ADMIN_USER=ironflyer-build
export UNATTENDED_BUILD=1
export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer
```

`IRONFLYER_MAC_POOL_ENABLED=1` is the signal `mobile.buildIOSNative`
and `mobile.buildFlutterIOS` look for; without it those drivers return
`ErrMacPoolDisabled` and the orchestrator surfaces a "iOS Pro tier
required" gate finding.

## Tart VM (optional — recommended for build isolation)

Running every build against the host keychain leaks signing material
across tenants. Tart lets you snapshot a clean Xcode-ready macOS VM and
fork it per build:

```bash
tart create --from-ipsw=latest ironflyer-base
tart run ironflyer-base &
# inside the VM: install Xcode + brew + tooling as above, then:
tart suspend ironflyer-base
```

The runtime then runs `tart clone ironflyer-base build-<id>` per build
and destroys the clone on teardown. This is the production path; bare-
metal builds without Tart are acceptable for solo-tenant deployments
only.

## Verification

From the runtime host:

```bash
ssh ironflyer-build@<mac-host> 'xcodebuild -version && xcodegen --version && tart --version'
```

When that returns a version triple, the Mac pool is wired correctly
and the orchestrator can flip an iOS-capable blueprint into the
ready-to-build state.
