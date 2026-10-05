<#
verify-msi-windows.ps1: installs hopto's MSI silently, as an unattended
install does, and checks what a person would look for afterwards: the exe
in its folder, the Start Menu shortcut, the entry in Installed apps with
its version, hopto running. With -Upgrade it installs a second, newer
package over the first while hopto runs, and checks the new version
replaced the old one, that a different hopto process is running and that
"open at login" survived. Then it uninstalls and checks that the program
is gone, that "open at login" is forgotten, that the rest of
%LocalAppData%\Programs is untouched and that the library is still there.

It installs into the profile it runs in, so it is meant for a test
machine, never for a PC somebody uses. Run it inside the Windows session.
Whatever fails midway, it leaves the machine as it found it: the package
uninstalled, the "open at login" value and its own test file removed,
hopto stopped.

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
$sentinel = Join-Path $env:LOCALAPPDATA 'Programs\hopto-verify-sentinel.txt'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'

# The "open at login" value as the app would write it.
$runValue = "`"$exe`""

# Runs msiexec without dialogs and returns its exit code; 0 is success.
function Installer([string]$arguments) {
    $process = Start-Process msiexec.exe -ArgumentList "$arguments /qn /norestart" -Wait -PassThru
    return $process.ExitCode
}

# The entry Installed apps shows for hopto, or $null. Windows Installer
# writes it under the user's hive or the machine's. On the test VM it wrote
# it under the machine's hive even for this per-user install, so both are
# read.
function InstalledEntry {
    $roots = @(
        'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )
    Get-ItemProperty $roots -ErrorAction SilentlyContinue |
        Where-Object { $_.DisplayName -eq 'hopto' }
}

# Waits up to $seconds for a hopto process started from the install folder
# whose id is not in $except, so a process that was already running before
# an update cannot be taken for the one the update started.
function WaitRunning([int]$seconds = 20, [int[]]$except = @()) {
    for ($i = 0; $i -lt $seconds * 5; $i++) {
        $running = Get-Process -Name hopto -ErrorAction SilentlyContinue |
            Where-Object { $_.Path -eq $exe -and $except -notcontains $_.Id }
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

# Whether the library was there before the first install. A library left by
# an earlier run cannot show that the first run seeded one, and nobody's
# library is deleted to force it.
$libraryExisted = Test-Path $library

StopHopto

try {
    Check 'the package installs without asking' ((Installer "/i `"$Msi`"") -eq 0)
    Check 'the exe is in its folder' (Test-Path $exe)
    Check 'the Start Menu has the shortcut' (Test-Path $shortcut)
    $entry = InstalledEntry
    Check 'Installed apps lists hopto' ($null -ne $entry)
    Check 'the entry names Drolosoft' ($entry.Publisher -eq 'Drolosoft')
    Check 'hopto starts by itself' (WaitRunning)
    $first = $entry.DisplayVersion

    # The library, and an "open at login" value as the app would have
    # written it, to see what an update and an uninstall do with each.
    if ($libraryExisted) {
        Check 'the library exists' (Test-Path $library)
    } else {
        Check 'the first run seeded the library' (Test-Path $library)
    }
    Set-ItemProperty $runKey -Name hopto -Value $runValue

    if ($Upgrade) {
        # hopto is running here on purpose: the update has to cope with it.
        $before = @(Get-Process -Name hopto -ErrorAction SilentlyContinue |
            Where-Object { $_.Path -eq $exe } |
            ForEach-Object { $_.Id })
        Check 'the update installs over a running hopto' ((Installer "/i `"$Upgrade`"") -eq 0)
        $entries = @(InstalledEntry)
        Check 'one entry is left, not two' ($entries.Count -eq 1)
        Check 'the version went up' ([version]$entries[0].DisplayVersion -gt [version]$first)
        Check 'hopto is running again, as a new process' (WaitRunning -except $before)
        $kept = (Get-ItemProperty $runKey -Name hopto -ErrorAction SilentlyContinue).hopto
        Check 'an update keeps open at login' ($kept -eq $runValue)
        Check 'the older package is now refused' ((Installer "/i `"$Msi`"") -ne 0)
    }

    # A file beside hopto's folder: an uninstall must not take the rest of
    # Programs with it.
    New-Item -ItemType File -Path $sentinel -Force | Out-Null

    # An empty product code would make msiexec open its usage window and
    # wait for somebody to close it, so without an entry the check fails
    # and msiexec is never called.
    $current = @(InstalledEntry) | Select-Object -First 1
    if ($current -and $current.PSChildName) {
        Check 'the package uninstalls' ((Installer "/x $($current.PSChildName)") -eq 0)
    } else {
        Check 'the package uninstalls: it is installed' $false
    }
    Check 'the exe is gone' (-not (Test-Path $exe))
    Check 'its folder is gone' (-not (Test-Path $folder))
    Check 'the shortcut is gone' (-not (Test-Path $shortcut))
    Check 'Installed apps no longer lists it' ($null -eq (InstalledEntry))
    Check 'open at login is forgotten' ($null -eq (Get-ItemProperty $runKey -Name hopto -ErrorAction SilentlyContinue))
    Check 'the rest of Programs is untouched' (Test-Path $sentinel)
    Check 'the library is still there' (Test-Path $library)
} catch {
    # A script that dies midway must not pass: it counts as a failure, and
    # the clean-up below still runs.
    Write-Output "✗ the script stopped: $_"
    $failures++
} finally {
    # Leaves the machine as it was found, whatever happened above.
    Remove-ItemProperty $runKey -Name hopto -ErrorAction SilentlyContinue
    Remove-Item $sentinel -Force -ErrorAction SilentlyContinue
    foreach ($leftover in @(InstalledEntry)) {
        if ($leftover.PSChildName) { [void](Installer "/x $($leftover.PSChildName)") }
    }
    StopHopto
}

if ($failures -gt 0) { exit 1 } else { exit 0 }
