# Installs the latest Woolly release on Windows.
#   irm https://usewoolly.com/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$repo = 'rjcuff/woolly'
$installDir = if ($env:WOOLLY_INSTALL_DIR) { $env:WOOLLY_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\woolly' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { throw "Unsupported CPU: $env:PROCESSOR_ARCHITECTURE" }
}

$file = "woolly_windows_$arch.zip"
$base = "https://github.com/$repo/releases/latest/download"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("woolly-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
  Write-Host "Downloading Woolly for windows/$arch..."
  Invoke-WebRequest "$base/$file" -OutFile (Join-Path $tmp $file) -UseBasicParsing
  Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

  # Make sure the download matches the checksum published with the release.
  $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match " $([regex]::Escape($file))$" }
  if (-not $line) { throw "No checksum published for $file. Nothing was installed." }
  $expected = ($line -split '\s+')[0].ToLower()
  $actual = (Get-FileHash (Join-Path $tmp $file) -Algorithm SHA256).Hash.ToLower()
  if ($expected -ne $actual) { throw "The download doesn't match the published checksum. Nothing was installed." }
  Write-Host 'Verified checksum.'

  Expand-Archive (Join-Path $tmp $file) -DestinationPath $tmp -Force
  New-Item -ItemType Directory -Path $installDir -Force | Out-Null
  Copy-Item (Join-Path $tmp 'woolly.exe') (Join-Path $installDir 'woolly.exe') -Force
}
finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# Add the install folder to your PATH for new terminals.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
  [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ";$installDir").TrimStart(';'), 'User')
  $env:Path = "$env:Path;$installDir"
  Write-Host "Added $installDir to your PATH. Open a new terminal to use it everywhere."
}

$version = & (Join-Path $installDir 'woolly.exe') version
Write-Host "Installed: $version"
Write-Host 'Next: woolly connect   (adds Woolly to Claude Code, Cursor, VS Code and your other AI apps)'
