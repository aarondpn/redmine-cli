# Installs the redmine CLI on Windows:
#
#   irm https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.ps1 | iex
#
# Environment variables:
#   REDMINE_CLI_VERSION           Release tag to install (e.g. v2.14.1). Defaults
#                                 to the latest release.
#   REDMINE_CLI_SKIP_ATTESTATION  1 or true skips the attestation check, e.g.
#                                 behind a proxy that blocks Sigstore. Not
#                                 recommended: only the checksums are verified.
#   REDMINE_INSTALL_DIR           Target directory. Defaults to
#                                 %LOCALAPPDATA%\Programs\redmine-cli.
#   REDMINE_CLI_NO_MODIFY_PATH    1 or true leaves the user PATH unchanged.
#
# Everything runs in a script block so that `irm | iex` neither leaks variables
# into the calling session nor closes it: failures are thrown, never `exit`ed.

& {
  $ErrorActionPreference = 'Stop'
  $ProgressPreference = 'SilentlyContinue'

  $Repo = 'aarondpn/redmine-cli'
  $Binary = 'redmine.exe'

  function Write-Info([string]$Message) {
    Write-Host '==> ' -ForegroundColor Blue -NoNewline
    Write-Host $Message
  }

  function Write-Warn([string]$Message) {
    Write-Host 'warning: ' -ForegroundColor Yellow -NoNewline
    Write-Host $Message
  }

  function Test-Truthy([string]$Value) {
    $Value -in @('1', 'true')
  }

  # Runs a web request, retrying transient failures such as Windows
  # PowerShell 5.1's "The connection was closed unexpectedly". Client errors
  # (4xx, e.g. a missing release or a rate limit) fail immediately.
  function Invoke-WithRetry([scriptblock]$Request) {
    for ($attempt = 1; ; $attempt++) {
      try {
        return & $Request
      } catch {
        $status = 0
        if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
        if ($attempt -ge 3 -or ($status -ge 400 -and $status -lt 500)) { throw }
        Write-Warn "Request failed ($($_.Exception.Message)); retrying..."
        Start-Sleep -Seconds (2 * $attempt)
      }
    }
  }

  # Runs gh with its output discarded and reports success. Native stderr must
  # not become a terminating error under ErrorActionPreference=Stop, which
  # Windows PowerShell 5.1 does for redirected stderr.
  function Invoke-Gh {
    $ErrorActionPreference = 'Continue'
    & gh @args *> $null
    $LASTEXITCODE -eq 0
  }

  # gh 2.49 added 'gh attestation', but --signer-workflow and --source-ref came
  # later. Without them a valid release would fail verification, so treat such
  # a gh like a missing one instead of reporting a security incident.
  function Test-GhVerifySupport {
    $ErrorActionPreference = 'Continue'
    $help = (& gh attestation verify --help 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) { return $false }
    ($help -match '(?m)^\s*--signer-workflow\s') -and ($help -match '(?m)^\s*--source-ref\s')
  }

  if ($PSVersionTable.PSEdition -eq 'Core' -and -not $IsWindows) {
    throw "install.ps1 installs the Windows build. On Linux and macOS run: curl -fsSL https://raw.githubusercontent.com/$Repo/main/install.sh | bash"
  }

  # A 32-bit PowerShell on 64-bit Windows reports the real CPU in
  # PROCESSOR_ARCHITEW6432.
  $Cpu = $env:PROCESSOR_ARCHITEW6432
  if (-not $Cpu) { $Cpu = $env:PROCESSOR_ARCHITECTURE }
  switch ($Cpu) {
    'AMD64' { $Arch = 'amd64' }
    # There is no Windows ARM64 build; Windows 11 on ARM runs the x64 one.
    'ARM64' { $Arch = 'amd64' }
    default { throw "Unsupported architecture: $Cpu. Download manually from https://github.com/$Repo/releases" }
  }
  Write-Info "Detected platform: windows/$Arch"

  # Windows PowerShell 5.1 may still default to TLS 1.0, which GitHub refuses.
  if ($PSVersionTable.PSVersion.Major -lt 6) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
  }

  $Tag = $env:REDMINE_CLI_VERSION
  if ($Tag) {
    # Pinned release: accept "2.14.1" as well as "v2.14.1".
    if (-not $Tag.StartsWith('v')) { $Tag = "v$Tag" }
    Write-Info "Requested version: $Tag"
  } else {
    Write-Info 'Fetching latest release...'
    $Tag = (Invoke-WithRetry { Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing }).tag_name
    if (-not $Tag) { throw 'Could not determine latest release' }
    Write-Info "Latest version: $Tag"
  }

  # The tag ends up in URLs and file names; only allow release-tag characters.
  # \A and \z, because $ would also match before a trailing newline.
  if ($Tag -cnotmatch '\Av[0-9A-Za-z][0-9A-Za-z.+-]*\z') {
    throw "Invalid release tag: $Tag"
  }

  $Version = $Tag.Substring(1)
  $Archive = "redmine-cli-windows-${Arch}.zip"
  $ChecksumsName = "redmine-cli_${Version}_checksums.txt"
  $BaseUrl = "https://github.com/$Repo/releases/download/$Tag"

  $TmpDir = Join-Path ([IO.Path]::GetTempPath()) ('redmine-cli-' + [Guid]::NewGuid())
  New-Item -ItemType Directory -Path $TmpDir | Out-Null
  try {
    $ArchivePath = Join-Path $TmpDir $Archive
    $ChecksumsPath = Join-Path $TmpDir 'checksums.txt'

    Write-Info "Downloading $BaseUrl/$Archive..."
    Invoke-WithRetry { Invoke-WebRequest -Uri "$BaseUrl/$Archive" -OutFile $ArchivePath -UseBasicParsing }
    Invoke-WithRetry { Invoke-WebRequest -Uri "$BaseUrl/$ChecksumsName" -OutFile $ChecksumsPath -UseBasicParsing }

    Write-Info 'Verifying checksum...'
    $Expected = $null
    foreach ($line in Get-Content -LiteralPath $ChecksumsPath) {
      $parts = $line.Trim() -split '\s+', 2
      if ($parts.Count -eq 2 -and $parts[1] -ceq $Archive) { $Expected = $parts[0] }
    }
    if (-not $Expected) { throw "No checksum found for $Archive" }
    $Actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArchivePath).Hash.ToLowerInvariant()
    if ($Actual -ne $Expected.ToLowerInvariant()) {
      throw "Checksum mismatch: expected $Expected, got $Actual"
    }
    Write-Info 'Checksum verified.'

    # Verify build provenance. The checksums come from the same release as the
    # archive, so only the attestation proves the archive was built by this
    # repository's release workflow for this tag.
    $ManualHint = "To verify this release yourself:`n`n" +
      "  Invoke-WebRequest $BaseUrl/$Archive -OutFile $Archive`n" +
      "  gh attestation verify $Archive --repo $Repo --signer-workflow $Repo/.github/workflows/release.yml --source-ref refs/tags/$Tag`n"
    $Skip = $null
    if (Test-Truthy $env:REDMINE_CLI_SKIP_ATTESTATION) {
      Write-Warn 'REDMINE_CLI_SKIP_ATTESTATION is set; skipping attestation verification (not recommended).'
      Write-Warn 'The archive was only checked against the release checksums, which come from the same place as the archive; a tampered release would NOT be detected.'
      Write-Host $ManualHint
    } elseif (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
      $Skip = 'GitHub CLI (gh) not found (https://cli.github.com)'
    } elseif (-not (Invoke-Gh attestation --help)) {
      $Skip = "Installed GitHub CLI does not support 'gh attestation' (needs gh 2.49+)"
    } elseif (-not $env:GH_TOKEN -and -not $env:GITHUB_TOKEN -and -not (Invoke-Gh auth status)) {
      $Skip = "GitHub CLI is not logged in (run 'gh auth login')"
    } elseif (-not (Test-GhVerifySupport)) {
      $Skip = "Installed GitHub CLI's 'gh attestation verify' lacks --signer-workflow or --source-ref (update gh)"
    } else {
      Write-Info 'Verifying attestation...'
      $verified = & {
        $ErrorActionPreference = 'Continue'
        & gh attestation verify $ArchivePath --repo $Repo --signer-workflow "$Repo/.github/workflows/release.yml" --source-ref "refs/tags/$Tag" | Out-Null
        $LASTEXITCODE -eq 0
      }
      if (-not $verified) {
        Write-Host ("If gh could not reach GitHub or Sigstore (e.g. a proxy blocks them, or gh is logged in to`n" +
          "GitHub Enterprise Server only), re-run with REDMINE_CLI_SKIP_ATTESTATION=1 to install with`n" +
          "checksum verification only (not recommended). If the attestation itself is invalid, do not`n" +
          "install; report it as a security issue. Releases older than v2.11.2 have no attestations;`n" +
          "install those with REDMINE_CLI_SKIP_ATTESTATION=1.")
        throw "Attestation verification failed for $Archive; refusing to install"
      }
      Write-Info 'Attestation verified.'
    }
    if ($Skip) {
      Write-Warn "$Skip; skipping attestation verification."
      Write-Host $ManualHint
    }

    $ExtractDir = Join-Path $TmpDir 'extract'
    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $ExtractDir
    $NewBinary = Join-Path $ExtractDir $Binary
    if (-not (Test-Path -LiteralPath $NewBinary -PathType Leaf)) {
      throw "$Binary not found in $Archive"
    }

    $InstallDir = $env:REDMINE_INSTALL_DIR
    if (-not $InstallDir) { $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\redmine-cli' }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $Target = Join-Path $InstallDir $Binary

    # A running redmine.exe cannot be overwritten, only renamed, so the old one
    # is moved aside first, as `redmine update` does.
    $Old = "$Target.old"
    if (Test-Path -LiteralPath $Target) {
      Remove-Item -LiteralPath $Old -Force -ErrorAction SilentlyContinue
      if (Test-Path -LiteralPath $Old) {
        throw "Could not remove $Old left by a previous update; close running redmine processes and retry"
      }
      Move-Item -LiteralPath $Target -Destination $Old
    }
    try {
      Move-Item -LiteralPath $NewBinary -Destination $Target
    } catch {
      if (Test-Path -LiteralPath $Old) { Move-Item -LiteralPath $Old -Destination $Target }
      throw
    }
    # Gone unless the previous binary is still running; the next update or
    # install removes it then.
    Remove-Item -LiteralPath $Old -Force -ErrorAction SilentlyContinue

    Write-Info "Installed redmine $Tag to $Target"
  } finally {
    Remove-Item -LiteralPath $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
  }

  # Add the install directory to the user PATH. The registry value is read and
  # written unexpanded so that entries such as %USERPROFILE%\bin survive.
  $RedmineCmd = 'redmine'
  $Normalized = $InstallDir.TrimEnd('\')
  $ModifyPath = -not (Test-Truthy $env:REDMINE_CLI_NO_MODIFY_PATH)
  $EnvKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $ModifyPath)
  try {
    $UserPath = [string]$EnvKey.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $Entries = @($UserPath -split ';' | Where-Object { $_ })
    $OnUserPath = @($Entries | Where-Object {
        [Environment]::ExpandEnvironmentVariables($_).TrimEnd('\') -eq $Normalized
      }).Count -gt 0

    if (-not $OnUserPath) {
      if (-not $ModifyPath) {
        $RedmineCmd = "& '$Target'"
        Write-Warn "$InstallDir is not in your PATH (REDMINE_CLI_NO_MODIFY_PATH is set)."
      } else {
        $EnvKey.SetValue('Path', (($Entries + $InstallDir) -join ';'), [Microsoft.Win32.RegistryValueKind]::ExpandString)
        # Writing the registry does not notify running programs; setting and
        # clearing a variable through .NET broadcasts WM_SETTINGCHANGE, so
        # terminals opened from Explorer see the new PATH.
        [Environment]::SetEnvironmentVariable('REDMINE_CLI_PATH_REFRESH', '1', 'User')
        [Environment]::SetEnvironmentVariable('REDMINE_CLI_PATH_REFRESH', $null, 'User')
        Write-Info "Added $InstallDir to your user PATH. Open a new terminal to use it everywhere."
      }
    }
  } finally {
    $EnvKey.Close()
  }
  if (@($env:Path -split ';' | Where-Object { $_.TrimEnd('\') -eq $Normalized }).Count -eq 0 -and $RedmineCmd -eq 'redmine') {
    # Usable in this session right away.
    $env:Path = "$env:Path;$InstallDir"
  }

  Write-Info 'Next: connect to your Redmine server with'
  Write-Host "      $RedmineCmd auth login"
  Write-Info 'Docs: https://redmine-cli.dev/getting-started/configuration/'
}
