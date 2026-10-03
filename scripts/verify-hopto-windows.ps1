<#
verify-hopto-windows.ps1: drives hopto.exe on Windows and checks what a
person would check by hand on a first run: the panel comes up by itself
with the welcome and the keyboard, Enter starts, both shortcuts register,
each one shows the panel with the keyboard in the page, the same shortcut
hides it, Esc hides it, the window has no taskbar button. The tray menu
is checked by hand (see doc/testing.md).

It runs on a profile of its own (a temporary USERPROFILE), never on
yours, so every run is a first run: the library is seeded and the welcome
shows. Run it inside the Windows session, not over SSH: synthetic keys
only reach the desktop they are sent from, and keys from keybd_event do
reach RegisterHotKey. It stops any hopto that is running before it starts
and does not start one again afterwards.

Usage: powershell -ExecutionPolicy Bypass -File scripts\verify-hopto-windows.ps1 -Exe C:\path\hopto.exe
Exit status: 0 every check passed, 1 a check failed, 3 the exe is missing.
#>
param([Parameter(Mandatory = $true)][string]$Exe)

# The marks below must survive a redirect to a file: Windows PowerShell 5.1
# would write them in the console code page and they would come out as "?".
# Without a console (a scheduled task, say) the property throws; the marks
# then come out as the host writes them.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8

if (-not (Test-Path $Exe)) { Write-Error "no exe at $Exe"; exit 3 }

Add-Type @"
using System; using System.Text; using System.Runtime.InteropServices;
public static class Native {
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
  [StructLayout(LayoutKind.Sequential)] public struct GUITHREADINFO {
    public int cbSize; public int flags; public IntPtr hwndActive; public IntPtr hwndFocus; public IntPtr hwndCapture;
    public IntPtr hwndMenuOwner; public IntPtr hwndMoveSize; public IntPtr hwndCaret; public RECT rcCaret; }
  [DllImport("user32.dll")] public static extern void keybd_event(byte vk, byte scan, uint flags, UIntPtr extra);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr FindWindow(string cls, string title);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll")] public static extern int GetWindowLong(IntPtr h, int i);
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  [DllImport("user32.dll")] public static extern bool GetGUIThreadInfo(uint tid, ref GUITHREADINFO info);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h, StringBuilder s, int n);
  // The class of the window that holds the keyboard focus, read from the
  // thread of the foreground window; empty when nothing has it.
  public static string FocusClass() {
    uint pid; uint tid = GetWindowThreadProcessId(GetForegroundWindow(), out pid);
    var info = new GUITHREADINFO(); info.cbSize = Marshal.SizeOf(info);
    if (!GetGUIThreadInfo(tid, ref info)) return "";
    var name = new StringBuilder(256); GetClassName(info.hwndFocus, name, 256);
    return name.ToString();
  }
}
"@

# Virtual keys (winuser.h).
$VK = @{Ctrl = 0x11; Shift = 0x10; Alt = 0x12; Space = 0x20; Esc = 0x1B; Enter = 0x0D}

# WS_EX_TOOLWINDOW, the style that keeps a window off the taskbar.
$ToolWindow = 0x80

# Presses the keys in order and releases them in reverse, as a hand does.
function Chord([int[]]$keys) {
    foreach ($k in $keys) { [Native]::keybd_event($k, 0, 0, [UIntPtr]::Zero) }
    [array]::Reverse($keys)
    foreach ($k in $keys) { [Native]::keybd_event($k, 0, 2, [UIntPtr]::Zero) }
    Start-Sleep -Milliseconds 800
}

# 1 with the panel shown, 0 hidden, -1 with no window at all.
function Visible {
    # $null reaches FindWindow as an empty string in PowerShell 5.1, which
    # asks for a window with an empty title, and the panel is titled
    # "hopto" (main.go), so the title is spelled out.
    $h = [Native]::FindWindow('hoptoWindow', 'hopto')
    if ($h -eq [IntPtr]::Zero) { return -1 }
    if ([Native]::IsWindowVisible($h)) { return 1 } else { return 0 }
}

# Waits up to $seconds for Visible to be $want.
function WaitVisible([int]$want, [int]$seconds = 3) {
    for ($i = 0; $i -lt $seconds * 5; $i++) {
        if ((Visible) -eq $want) { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

# Waits up to $seconds for the keyboard to reach the page: the panel is the
# foreground window and the focus sits in one of the Chromium widgets
# WebView2 keeps under it (Chrome_WidgetWin_1 when measured; any of them
# means Chromium, not the bare window, gets the keys). The foreground alone
# is not enough; a key typed while the focus is still on the bare window
# never reaches the page.
function WaitPageFocus([int]$seconds = 3) {
    for ($i = 0; $i -lt $seconds * 10; $i++) {
        $front = [Native]::GetForegroundWindow() -eq [Native]::FindWindow('hoptoWindow', 'hopto')
        if ($front -and [Native]::FocusClass().StartsWith('Chrome_WidgetWin')) { return $true }
        Start-Sleep -Milliseconds 100
    }
    return $false
}

# Waits up to $seconds for a line matching $pattern in hopto's log.
function WaitLog([string]$pattern, [int]$seconds = 3) {
    for ($i = 0; $i -lt $seconds * 5; $i++) {
        if ((Test-Path $log) -and ((Get-Content $log -Raw) -match $pattern)) { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

# Waits up to $seconds for the hopto window to exist, hidden or not: a cold
# WebView2 start on an empty profile can take well over five seconds.
function WaitWindow([int]$seconds = 20) {
    for ($i = 0; $i -lt $seconds * 5; $i++) {
        if ([Native]::FindWindow('hoptoWindow', 'hopto') -ne [IntPtr]::Zero) { return $true }
        Start-Sleep -Milliseconds 200
    }
    return $false
}

$failures = 0
function Check([string]$name, [bool]$ok) {
    if ($ok) { Write-Output "✓ $name" } else { Write-Output "✗ $name"; $script:failures++ }
}

# A profile of its own: the data folder and the log land under it.
$tempProfile = Join-Path $env:TEMP ("hopto-verify-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $tempProfile | Out-Null
$env:USERPROFILE = $tempProfile
$log = Join-Path $tempProfile 'AppData\Local\hopto\hopto.log'

# Stops every hopto build and waits until it is gone. The process is named
# after the file, so a build kept as hopto-new.exe runs as "hopto-new":
# stopping "hopto" alone leaves it alive, holding the hotkeys and
# answering the shortcuts instead of the build under test. Stop-Process
# returns before the process has died, and a new instance that starts
# first would meet the old single-instance lock.
function StopHopto {
    Get-Process -Name hopto* -ErrorAction SilentlyContinue |
        Stop-Process -Force -PassThru |
        Wait-Process -Timeout 5 -ErrorAction SilentlyContinue
}

StopHopto
Start-Process -FilePath $Exe

if (WaitWindow) {
    # A first run shows the panel by itself, with the welcome over it and
    # its Start button holding the keyboard; Enter closes the welcome and
    # leaves the panel on screen. Without the welcome, Enter would open the
    # first app of the seeded library, so it is only pressed once the log
    # says the welcome is up.
    $welcome = (WaitLog 'welcome: showing the panel' 20) -and (WaitVisible 1)
    Check 'the first run shows the panel with the welcome' $welcome
    if ($welcome) {
        Check 'the page has the keyboard' (WaitPageFocus)
        Chord @($VK.Enter)
        Check 'Enter closes the welcome and keeps the panel' ((WaitLog 'welcome: dismissed') -and ((Visible) -eq 1))
    }
    Chord @($VK.Esc); Check 'Esc hides it' (WaitVisible 0)

    Chord @($VK.Ctrl, $VK.Shift, $VK.Space); Check 'Ctrl+Shift+Space shows the panel' (WaitVisible 1)
    Check 'the page has the keyboard again' (WaitPageFocus)
    Chord @($VK.Ctrl, $VK.Shift, $VK.Space); Check 'Ctrl+Shift+Space again hides it' (WaitVisible 0)
    Chord @($VK.Ctrl, $VK.Alt, $VK.Space); Check 'Ctrl+Alt+Space shows the links' (WaitVisible 1)
    Check 'the links page has the keyboard' (WaitPageFocus)
    Chord @($VK.Esc); Check 'Esc hides the links' (WaitVisible 0)

    $h = [Native]::FindWindow('hoptoWindow', 'hopto')
    $style = [Native]::GetWindowLong($h, -20)
    Check 'no taskbar button (WS_EX_TOOLWINDOW)' (($style -band $ToolWindow) -ne 0)

    $text = if (Test-Path $log) { Get-Content $log -Raw } else { '' }
    Check 'hotkey 1 registered' ($text -match 'hotkey 1 registered')
    Check 'hotkey 2 registered' ($text -match 'hotkey 2 registered')
    Check 'the tray icon was added' ($text -match 'tray icon added')
    Check 'library.toml was seeded' (Test-Path (Join-Path $tempProfile 'AppData\Roaming\hopto\library.toml'))
} else {
    Check 'hopto did not start within 20 s' $false
}

StopHopto
Remove-Item -Recurse -Force $tempProfile -ErrorAction SilentlyContinue

if ($failures -gt 0) { exit 1 } else { exit 0 }
