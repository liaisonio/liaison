#requires -Version 5.1
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [ValidatePattern('^[a-f0-9]{32}$')][string]$InstanceId,
    [switch]$KeepConfig,
    [switch]$KeepLogs,
    [switch]$Help
)
$ErrorActionPreference = 'Stop'
if ($Help) { Write-Output 'uninstall.ps1 -InstanceId <id> [-KeepConfig] [-KeepLogs] [-WhatIf]'; exit 0 }
try {
    if (-not $InstanceId) { throw 'An explicit InstanceId is required. Legacy installations are not automatically removed; no processes were stopped.' }
    $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Open PowerShell as administrator.' }
    $name = 'LiaisonEdge-' + $InstanceId
    $base = Join-Path $env:ProgramFiles ('Liaison\edges\' + $InstanceId)
    for ($path = $base; $path; $path = Split-Path -Parent $path) {
        if ((Test-Path -LiteralPath $path) -and ((Get-Item -LiteralPath $path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Refusing a path containing a reparse point.' }
    }
    $bin = Join-Path $base 'liaison-edge.exe'
    $config = Join-Path $base 'config.yaml'
    $expected = '"' + $bin + '" --edge-windows-service ' + $InstanceId + ' -c "' + $config + '"'
    $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='$name'"
    if (-not $service -or $service.PathName -cne $expected -or $service.StartName -ne 'NT AUTHORITY\LocalService') { throw 'Service ownership could not be verified. Nothing was removed.' }
    if (-not $PSCmdlet.ShouldProcess($name, 'Stop and remove this Edge instance')) { return }
    Stop-Service -Name $name
    (Get-Service -Name $name).WaitForStatus('Stopped', [TimeSpan]::FromSeconds(35))
    $users = @(Get-CimInstance -ClassName Win32_Process -Filter "Name='liaison-edge.exe'" | Where-Object { $_.ExecutablePath -eq $bin })
    if ($users.Count) { throw 'The instance executable is still in use. Files were retained.' }
    & "$env:SystemRoot\System32\sc.exe" delete $name | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Service removal failed; files were retained.' }
    # Delete known leaves only. Shared directories and One are never targets.
    foreach ($leaf in @('liaison-edge.exe', 'config.yaml', 'ca.pem')) {
        if ($KeepConfig -and $leaf -ne 'liaison-edge.exe') { continue }
        $target = Join-Path $base $leaf
        if (Test-Path -LiteralPath $target) { Remove-Item -LiteralPath $target -Force }
    }
    if (-not $KeepLogs) {
        $logs = Join-Path $base 'logs'
        if (Test-Path -LiteralPath $logs) {
            if ((Get-Item -LiteralPath $logs -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Log directory is a reparse point; retained.' }
            Get-ChildItem -LiteralPath $logs -File | Where-Object { -not ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) } | Remove-Item -Force
            if (-not @(Get-ChildItem -LiteralPath $logs -Force).Count) { Remove-Item -LiteralPath $logs }
        }
    }
    if (-not @(Get-ChildItem -LiteralPath $base -Force).Count) { Remove-Item -LiteralPath $base }
    Write-Output "Removed Edge instance $InstanceId. Other instances were not modified."
} catch {
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
}
