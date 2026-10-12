#requires -Version 5.1
[CmdletBinding()]
param(
    [string]$AccessKey,
    [string]$SecretKey,
    [string]$ServerHttpAddr,
    [string]$ServerEdgeAddr,
    [string]$CACert,
    [string]$ExpectedSHA256,
    [string]$PackagePath,
    [switch]$NewInstance,
    [switch]$Help
)
$ErrorActionPreference = 'Stop'
if ($Help) {
    Write-Output 'install.ps1 -AccessKey <key> -SecretKey <secret> -ServerHttpAddr <https-url> -ServerEdgeAddr <host:port> [-CACert <pem>] [-ExpectedSHA256 <sha256>]'
    exit 0
}
function Assert-SafePath([string]$Path) {
    for ($current = $Path; $current; $current = Split-Path -Parent $current) {
        if ((Test-Path -LiteralPath $current) -and ((Get-Item -LiteralPath $current -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw 'Refusing a path containing a reparse point.'
        }
    }
}
function Set-PrivateACL([string]$Path, [bool]$ServiceWrite, [string]$ServiceSID = '') {
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($sid in @('S-1-5-18', 'S-1-5-32-544')) {
        $rule = New-Object Security.AccessControl.FileSystemAccessRule(
            (New-Object Security.Principal.SecurityIdentifier($sid)), 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    if ($ServiceSID) {
        $rights = if ($ServiceWrite) { 'Modify' } else { 'ReadAndExecute' }
        $rule = New-Object Security.AccessControl.FileSystemAccessRule(
            (New-Object Security.Principal.SecurityIdentifier($ServiceSID)), $rights, 'ContainerInherit,ObjectInherit', 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}
function Yaml-String([string]$Value) { return "'" + $Value.Replace("'", "''") + "'" }
$temp = $null
$serviceCreated = $false
$completed = $false
try {
    $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Administrator permission is required for a Windows service. Open PowerShell as administrator, or use Liaison One.'
    }
    if (-not [Environment]::Is64BitOperatingSystem) { throw 'This package requires 64-bit Windows.' }
    foreach ($value in @($AccessKey, $SecretKey, $ServerEdgeAddr)) {
        if ([string]::IsNullOrWhiteSpace($value) -or $value.Length -gt 4096 -or $value -match "[\r\n\x00]") { throw 'Missing or invalid enrollment fields.' }
    }
    if ($ServerEdgeAddr -notmatch '^(\[[0-9a-fA-F:]+\]|[^\s:/]+):([0-9]{1,5})$' -or [int]$Matches[2] -lt 1 -or [int]$Matches[2] -gt 65535) { throw 'ServerEdgeAddr must be host:port.' }
    if (-not $PackagePath) {
        if ($ServerHttpAddr -notmatch '^https?://') { $ServerHttpAddr = 'https://' + $ServerHttpAddr }
        $uri = [Uri]$ServerHttpAddr
        if (-not $uri.IsAbsoluteUri -or $uri.Scheme -ne 'https' -or $uri.UserInfo -or $uri.Query -or $uri.Fragment) { throw 'A trusted HTTPS download URL is required.' }
    } elseif (-not $ExpectedSHA256) { throw 'Offline packages require -ExpectedSHA256 from a trusted release manifest.' }
    if ($ExpectedSHA256 -and $ExpectedSHA256 -notmatch '^[a-fA-F0-9]{64}$') { throw 'Invalid expected SHA-256.' }
    if ($CACert) { $CACert = (Resolve-Path -LiteralPath $CACert).Path }
    $tar = (Get-Command tar.exe -ErrorAction Stop).Source
    $id = [Guid]::NewGuid().ToString('N')
    $serviceName = 'LiaisonEdge-' + $id
    $base = Join-Path $env:ProgramFiles 'Liaison\edges'
    Assert-SafePath $base
    $instanceDir = Join-Path $base $id
    if (Test-Path -LiteralPath $instanceDir) { throw 'Instance already exists; no files were overwritten.' }
    $temp = Join-Path ([IO.Path]::GetTempPath()) ('liaison-edge-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temp | Out-Null
    Set-PrivateACL $temp $false
    $archive = Join-Path $temp 'edge.tar.gz'
    if ($PackagePath) {
        Copy-Item -LiteralPath $PackagePath -Destination $archive
    } else {
        $curl = (Get-Command curl.exe -ErrorAction Stop).Source
        $url = $ServerHttpAddr.TrimEnd('/') + '/packages/edge/liaison-edge-windows-amd64.tar.gz'
        Write-Output 'Downloading Windows Edge package...'
        $downloadArgs = @('--fail', '--silent', '--show-error', '--location', '--proto', '=https', '--proto-redir', '=https', '--connect-timeout', '15', '--max-time', '180')
        if ($CACert) { $downloadArgs += @('--cacert', $CACert) }
        & $curl @downloadArgs --output $archive $url
        if ($LASTEXITCODE -ne 0) { throw 'Package download failed. Check connectivity and certificate trust; use -CACert for your private CA.' }
    }
    if ($ExpectedSHA256 -and (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $ExpectedSHA256) { throw 'Package checksum mismatch.' }
    $entries = @(& $tar -tzf $archive)
    if ($LASTEXITCODE -ne 0) { throw 'Package archive is invalid.' }
    foreach ($entry in $entries) {
        if ($entry -notmatch '^(\./)?liaison-edge(\.exe|\.yaml\.template)$') { throw 'Unexpected package contents.' }
    }
    & $tar -xzf $archive -C $temp
    if ($LASTEXITCODE -ne 0) { throw 'Package extraction failed.' }
    $source = Join-Path $temp 'liaison-edge.exe'
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw 'Package does not contain Edge.' }
    Assert-SafePath $source
    New-Item -ItemType Directory -Path $instanceDir | Out-Null
    Set-PrivateACL $instanceDir $false
    $bin = Join-Path $instanceDir 'liaison-edge.exe'
    $conf = Join-Path $instanceDir 'config.yaml'
    $logs = Join-Path $instanceDir 'logs'
    New-Item -ItemType Directory -Path $logs | Out-Null
    Set-PrivateACL $logs $true
    Copy-Item -LiteralPath $source -Destination $bin
    $caLine = ''
    if ($CACert) {
        $installedCA = Join-Path $instanceDir 'ca.pem'
        Copy-Item -LiteralPath $CACert -Destination $installedCA
        $caLine = '      ca_certs: [' + (Yaml-String $installedCA.Replace('\', '/')) + ']'
    }
    $config = @(
        ('instance_id: ' + (Yaml-String $id))
        'allow_remote_uninstall: false'
        'manager:'
        '  dial:'
        ('    addrs: [' + (Yaml-String $ServerEdgeAddr) + ']')
        '    network: tcp'
        '    tls:'
        '      enable: true'
        '      insecure_skip_verify: false'
        $caLine
        '  auth:'
        ('    access_key: ' + (Yaml-String $AccessKey))
        ('    secret_key: ' + (Yaml-String $SecretKey))
        'log:'
        '  level: info'
        ('  file: ' + (Yaml-String (Join-Path $logs 'edge.log').Replace('\', '/')))
        '  maxsize: 20'
        '  maxrolls: 5'
    )
    [IO.File]::WriteAllLines($conf, $config, (New-Object Text.UTF8Encoding($false)))
    $command = '"' + $bin + '" --edge-windows-service ' + $id + ' -c "' + $conf + '"'
    $account = New-Object Management.Automation.PSCredential('NT AUTHORITY\LocalService', (New-Object Security.SecureString))
    New-Service -Name $serviceName -BinaryPathName $command -DisplayName ('Liaison Edge ' + $id) -StartupType Automatic -Credential $account | Out-Null
    $serviceCreated = $true
    & "$env:SystemRoot\System32\sc.exe" sidtype $serviceName unrestricted | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Failed to enable the instance service identity.' }
    $serviceIdentity = New-Object Security.Principal.NTAccount('NT SERVICE', $serviceName)
    $serviceSID = $serviceIdentity.Translate([Security.Principal.SecurityIdentifier]).Value
    Set-PrivateACL $instanceDir $false $serviceSID
    Set-PrivateACL $logs $true $serviceSID
    & "$env:SystemRoot\System32\sc.exe" failure $serviceName reset= 86400 actions= restart/5000/restart/15000/restart/60000 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Failed to configure service recovery.' }
    Start-Service -Name $serviceName
    (Get-Service -Name $serviceName).WaitForStatus('Running', [TimeSpan]::FromSeconds(35))
    Start-Sleep -Seconds 2
    if ((Get-Service -Name $serviceName).Status -ne 'Running') { throw 'Edge exited during startup.' }
    $completed = $true
    Write-Output "Instance: $id"
    Write-Output "Service: $serviceName"
    Write-Output 'Service installed and running. Confirm connector online status in the platform; running is not proof of a connected tunnel.'
} catch {
    Write-Error ('Windows Edge installation failed: ' + $_.Exception.Message) -ErrorAction Continue
    exit 1
} finally {
    if (-not $completed -and $serviceCreated) {
        # Preserve failed instance files for diagnosis; do not recursively delete
        # a directory while the service could still be using it.
        Stop-Service -Name $serviceName -ErrorAction SilentlyContinue
        & "$env:SystemRoot\System32\sc.exe" delete $serviceName | Out-Null
    }
    if ($temp -and (Test-Path -LiteralPath $temp)) { Remove-Item -LiteralPath $temp -Recurse -Force }
}
