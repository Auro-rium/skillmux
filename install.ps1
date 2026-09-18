$ErrorActionPreference = "Stop"

$Repo = "Auro-rium/skillmux"
$Version = if ($env:SKILLMUX_VERSION) { $env:SKILLMUX_VERSION } else { "latest" }
$BinDir = if ($env:SKILLMUX_BIN_DIR) { $env:SKILLMUX_BIN_DIR } else { Join-Path $HOME ".local\bin" }

$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
if ($arch -ne "X64") {
    throw "skillmux: unsupported Windows architecture '$arch' (V1 publishes windows/amd64)"
}

$archive = "skillmux_windows_amd64.zip"

if ($Version -eq "latest") {
    $baseUrl = "https://github.com/$Repo/releases/latest/download"
} else {
    if (-not $Version.StartsWith("v")) { $Version = "v$Version" }
    $baseUrl = "https://github.com/$Repo/releases/download/$Version"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("skillmux-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    $archivePath = Join-Path $tmp $archive
    $checksumsPath = Join-Path $tmp "checksums.txt"

    Write-Host "Downloading $baseUrl/$archive"
    Invoke-WebRequest -UseBasicParsing "$baseUrl/$archive" -OutFile $archivePath
    Invoke-WebRequest -UseBasicParsing "$baseUrl/checksums.txt" -OutFile $checksumsPath

    $line = Get-Content $checksumsPath | Where-Object { $_ -match [regex]::Escape($archive) } | Select-Object -First 1
    if (-not $line) {
        throw "skillmux: checksum entry for $archive was not found"
    }

    $expected = ($line -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 $archivePath).Hash.ToLowerInvariant()
    if ($expected -ne $actual) {
        throw "skillmux: checksum verification failed for $archive"
    }

    $extract = Join-Path $tmp "extract"
    Expand-Archive -Path $archivePath -DestinationPath $extract -Force

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $destination = Join-Path $BinDir "skillmux.exe"
    Copy-Item -Force (Join-Path $extract "skillmux.exe") $destination

    Write-Host "Installed skillmux to $destination"
    & $destination version

    $pathEntries = $env:Path -split ";"
    if ($pathEntries -notcontains $BinDir) {
        Write-Host ""
        Write-Host "$BinDir is not currently on PATH."
        Write-Host "Add it to your user PATH, or run:"
        Write-Host "  $destination"
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
