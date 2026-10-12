# Native, disposable Windows runner only. Does not contact a Liaison server.
# Exercise the actual PowerShell 5.1 installer and SCM with a no-network fixture.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$temp = Join-Path $env:TEMP ('liaison-test-' + [Guid]::NewGuid().ToString('N'))
$instances = New-Object Collections.Generic.List[string]
function Assert([bool]$Condition, [string]$Message) { if (-not $Condition) { throw $Message } }
try {
    New-Item -ItemType Directory -Path $temp | Out-Null
    $exe = Join-Path $temp 'liaison-edge.exe'
    & go build -o $exe ./internal/testutil/windows-edge-service
    Assert ($LASTEXITCODE -eq 0) 'Fixture build failed'
    $archive = Join-Path $temp 'edge.tar.gz'
    & tar.exe -czf $archive -C $temp liaison-edge.exe
    Assert ($LASTEXITCODE -eq 0) 'Fixture packaging failed'
    $hash = (Get-FileHash $archive -Algorithm SHA256).Hash
    $install = Join-Path $root 'dist\edge\install.ps1'
    $uninstall = Join-Path $root 'dist\edge\uninstall.ps1'
    foreach ($index in 1..2) {
        $output = & powershell.exe -NoProfile -File $install -AccessKey fixture-key -SecretKey fixture-not-a-secret -ServerEdgeAddr localhost:443 -PackagePath $archive -ExpectedSHA256 $hash
        Assert ($LASTEXITCODE -eq 0) 'Install failed'
        $line = @($output | Where-Object { $_ -match '^Instance: ([a-f0-9]{32})$' })
        Assert ($line.Count -eq 1) 'Missing instance result'
        $id = $line[0].Substring(10)
        $instances.Add($id)
        Assert ((Get-Service ('LiaisonEdge-' + $id)).Status -eq 'Running') 'Service not running'
        $conf = Join-Path $env:ProgramFiles ('Liaison\edges\' + $id + '\config.yaml')
        $text = Get-Content -LiteralPath $conf -Raw
        Assert ($text.Contains('insecure_skip_verify: false')) 'TLS verification disabled'
        Assert ($text.Contains('allow_remote_uninstall: false')) 'Remote uninstall enabled before verification'
        $acl = Get-Acl -LiteralPath $conf
        Assert (-not @($acl.Access | Where-Object { $_.IdentityReference.Value -match 'Everyone|BUILTIN\\Users|Authenticated Users' }).Count) 'Broad credential access'
        $identities = @($acl.Access | ForEach-Object { $_.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value })
        Assert ($identities -notcontains 'S-1-5-19') 'Shared LocalService identity can read instance credentials'
        $identity = New-Object Security.Principal.NTAccount('NT SERVICE', ('LiaisonEdge-' + $id))
        Assert ($identities -contains $identity.Translate([Security.Principal.SecurityIdentifier]).Value) 'Missing instance-specific read permission'
        Assert (Test-Path (Join-Path (Split-Path $conf) 'logs\fixture.log')) 'Service cannot write logs'
    }
    Assert ($instances[0] -ne $instances[1]) 'Instances collided'
    # Missing selection must fail without stopping either instance.
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & powershell.exe -NoProfile -File $uninstall 2>$null
        $ambiguousExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $previousPreference }
    Assert ($ambiguousExit -ne 0) 'Ambiguous uninstall accepted'
    Assert ((Get-Service ('LiaisonEdge-' + $instances[1])).Status -eq 'Running') 'Other instance stopped'
    & $uninstall -InstanceId $instances[0] -Confirm:$false
    Assert (-not (Get-Service ('LiaisonEdge-' + $instances[0]) -ErrorAction SilentlyContinue)) 'Selected service remains'
    Assert ((Get-Service ('LiaisonEdge-' + $instances[1])).Status -eq 'Running') 'Other instance was modified'
    Write-Output 'PASS: native SCM startup, isolated install, ACL, TLS defaults, precise uninstall'
} finally {
    foreach ($id in $instances) {
        $name = 'LiaisonEdge-' + $id
        if (Get-Service $name -ErrorAction SilentlyContinue) { & $uninstall -InstanceId $id -Confirm:$false }
    }
    if (Test-Path $temp) { Remove-Item -LiteralPath $temp -Recurse -Force }
}
