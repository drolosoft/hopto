<#
verify-msi-windows.ps1: installs hopto's MSI the way a double click does,
without the dialogs, and checks what a person would look for afterwards:
the exe in its folder, the Start Menu shortcut, the entry in Installed
apps with its version, hopto running. With -Upgrade it installs a second,
newer package over the first while hopto runs, and checks the new version
replaced the old one. Then it uninstalls and checks that the program is
gone and that the library is still there.

It installs into the profile it runs in, so it is meant for a test
machine, never for a PC somebody uses. Run it inside the Windows session.

Usage: powershell -ExecutionPolicy Bypass -File scripts\verify-msi-windows.ps1 -Msi C:\path\hopto.msi [-Upgrade C:\path\newer.msi]
Exit status: 0 every check passed, 1 a check failed, 3 a package is missing.
#>
param(
    [Parameter(Mandatory = $true)][string]$Msi,
    [string]$Upgrade = ''
)

try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8

foreach ($package in @($Msi, $Upgrade)) {
    if ($package -and -not (Test-Path $package)) { Write-Error "no package at $package"; exit 3 }
}

$failures = 0
function Check([string]$name, [bool]$ok) {
    if ($ok) { Write-Output "✓ $name" } else { Write-Output "✗ $name"; $script:failures++ }
}

# Where the package installs, and what it must leave alone.
$folder = Join-Path $env:LOCALAPPDATA 'Programs\hopto'
$exe = Join-Path $folder 'hopto.exe'
$shortcut = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\hopto.lnk'
$library = Join-Path $env:APPDATA 'hopto\library.toml'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'

# Runs msiexec without dialogs and returns its exit code; 0 is success.
function Installer([string]$arguments) {
    $process = Start-Process msiexec.exe -ArgumentList "$arguments /qn /norestart" -Wait -PassThru
    return $process.ExitCode
}

# The entry Installed apps shows for hopto, or $null. Windows Installer
# writes it under the user's hive or the machine's depending on how the
# package was started, so both are read.
function InstalledEntry {
    $roots = @(
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    Get-ItemProperty $roots -ErrorAction SilentlyContinue |
        Where-Object { $_.DisplayName -eq 'hopto' }
}

# Waits up to $seconds for a hopto process started from the install folder.
function WaitRunning([int]$seconds = 20) {
    for ($i = 0; $i -lt $seconds * 5; $i++) {
        $running = Get-Process -Name hopto -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe }
        if ($running) { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

function StopHopto {
    Get-Process -Name hopto* -ErrorAction SilentlyContinue |
        Stop-Process -Force -PassThru |
        Wait-Process -Timeout 5 -ErrorAction SilentlyContinue
}

StopHopto

Check 'the package installs without asking' ((Installer "/i `"$Msi`"") -eq 0)
Check 'the exe is in its folder' (Test-Path $exe)
Check 'the Start Menu has the shortcut' (Test-Path $shortcut)
$entry = InstalledEntry
Check 'Installed apps lists hopto' ($null -ne $entry)
Check 'the entry names Drolosoft' ($entry.Publisher -eq 'Drolosoft')
Check 'hopto starts by itself' (WaitRunning)
$first = $entry.DisplayVersion

if ($Upgrade) {
    # hopto is running here on purpose: the update has to cope with it.
    Check 'the update installs over a running hopto' ((Installer "/i `"$Upgrade`"") -eq 0)
    $entries = @(InstalledEntry)
    Check 'one entry is left, not two' ($entries.Count -eq 1)
    Check 'the version went up' ([version]$entries[0].DisplayVersion -gt [version]$first)
    Check 'hopto is running again' (WaitRunning)
    Check 'the older package is now refused' ((Installer "/i `"$Msi`"") -ne 0)
}

# The library the first run seeded, and an "open at login" value as the
# app would have written it, to see what an uninstall does with each.
Check 'the first run seeded the library' (Test-Path $library)
Set-ItemProperty $runKey -Name hopto -Value "`"$exe`""

$current = (InstalledEntry).PSChildName
Check 'the package uninstalls' ((Installer "/x $current") -eq 0)
Check 'the exe is gone' (-not (Test-Path $exe))
Check 'its folder is gone' (-not (Test-Path $folder))
Check 'the shortcut is gone' (-not (Test-Path $shortcut))
Check 'Installed apps no longer lists it' ($null -eq (InstalledEntry))
Check 'open at login is forgotten' ($null -eq (Get-ItemProperty $runKey -Name hopto -ErrorAction SilentlyContinue))
Check 'the library is still there' (Test-Path $library)

StopHopto

if ($failures -gt 0) { exit 1 } else { exit 0 }
