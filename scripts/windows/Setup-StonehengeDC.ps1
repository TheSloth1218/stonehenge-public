# Run on a fresh Windows Server 2022/2025 VM as Administrator.
# Promote sets the FCU-DC hostname and creates fcu.gov. Configure seeds the lab.
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('Promote', 'Configure')]
    [string]$Phase,
    [string]$AppHostAddress
)

$ErrorActionPreference = 'Stop'
$root = 'C:\ProgramData\Stonehenge'
$statePath = Join-Path $root 'dc-secrets.json'
$marker = Join-Path $root 'configured.txt'

$os = (Get-CimInstance Win32_OperatingSystem).Caption
if ($os -notmatch 'Windows Server (2022|2025)') {
    throw 'This script expects Windows Server 2022 or 2025.'
}
$admin = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $admin.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run in an elevated PowerShell session.'
}

if ($Phase -eq 'Promote') {
    $computerSystem = Get-CimInstance Win32_ComputerSystem
    if ($computerSystem.PartOfDomain -and $computerSystem.Domain -ne 'fcu.gov') {
        throw 'This server already belongs to another domain.'
    }
    if ($computerSystem.PartOfDomain -and $env:COMPUTERNAME -ne 'FCU-DC') {
        throw 'An existing domain controller has a different hostname.'
    }
    if ($env:COMPUTERNAME -ne 'FCU-DC') {
        Rename-Computer -NewName 'FCU-DC' -Force -Restart
        return
    }
    Install-WindowsFeature AD-Domain-Services -IncludeManagementTools | Out-Null
    Import-Module ADDSDeployment
    $existing = Get-ADDomain -ErrorAction SilentlyContinue
    if ($null -ne $existing) {
        if ($existing.DNSRoot -ne 'fcu.gov') { throw 'An unrelated domain already exists.' }
        Write-Host 'fcu.gov already exists; promotion skipped.'
        return
    }
    $dsrm = Read-Host 'Enter and separately record the DSRM password' -AsSecureString
    Install-ADDSForest -DomainName fcu.gov -DomainNetbiosName FCU `
        -InstallDns -SafeModeAdministratorPassword $dsrm -Force
    return
}

if (-not $AppHostAddress) { throw 'Configure requires -AppHostAddress with the Docker host IPv4 address.' }
$parsedAddress = $null
if (-not [Net.IPAddress]::TryParse($AppHostAddress, [ref]$parsedAddress) -or
    $parsedAddress.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) {
    throw 'AppHostAddress must be an IPv4 address.'
}
Import-Module ActiveDirectory
Import-Module DnsServer
$domain = Get-ADDomain -ErrorAction Stop
if ($domain.DNSRoot -ne 'fcu.gov') { throw 'This script only configures the isolated fcu.gov forest.' }
if (@(Get-ADDomainController -Filter *).Count -ne 1) {
    throw 'Use a single-DC disposable lab; this script changes directory security settings.'
}
if (Test-Path $marker) {
    Write-Host 'Stonehenge DC is already configured; existing accounts and student changes were preserved.'
    return
}

New-Item -ItemType Directory -Path $root -Force | Out-Null
function New-LabPassword {
    $bytes = [byte[]]::new(24)
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $generator.GetBytes($bytes) }
    finally { $generator.Dispose() }
    return ([Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_') + '!aA1')
}

if (Test-Path $statePath) {
    $secrets = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
} else {
    $users = @{}
    foreach ($name in @('chopper','grim','edge','count','swordsman','mobius1','blaze','cipher','trigger')) {
        $users[$name] = New-LabPassword
    }
    $secrets = [ordered]@{
        users = $users
        dexBind = New-LabPassword
        vendorArchive = New-LabPassword
        flightOps = New-LabPassword
    }
    $secrets | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $statePath -Encoding UTF8
    & icacls.exe $statePath /inheritance:r /grant:r '*S-1-5-32-544:F' '*S-1-5-18:F' | Out-Null
}

$groupMembers = [ordered]@{
    'SCA-Employees' = @('chopper','grim','edge','count','swordsman','mobius1','blaze','cipher','trigger')
    'SCA-Admins' = @('edge')
    'SCA-Engineering-Users' = @('count','swordsman','mobius1','blaze','cipher','trigger')
    'SCA-Engineering-Admins' = @('mobius1','blaze','cipher','trigger')
}
foreach ($groupName in $groupMembers.Keys) {
    if (-not (Get-ADGroup -Identity $groupName -ErrorAction SilentlyContinue)) {
        New-ADGroup -Name $groupName -GroupScope Global -GroupCategory Security `
            -Path 'CN=Users,DC=fcu,DC=gov' | Out-Null
    }
}

$staff = @()
foreach ($name in $groupMembers['SCA-Employees']) {
    $staff += [ordered]@{username = $name}
    if (-not (Get-ADUser -Identity $name -ErrorAction SilentlyContinue)) {
        $description = if ($name -eq 'chopper') {
            "Civil Service Employee | Temporary password: $($secrets.users.$name)"
        } else { 'Stonehenge lab staff' }
        New-ADUser -Name $name -SamAccountName $name -UserPrincipalName "$name@fcu.gov" `
            -DisplayName "FCU $name" -EmailAddress "$name@fcu.gov" `
            -Description $description -Path 'CN=Users,DC=fcu,DC=gov' -Enabled $true `
            -AccountPassword (ConvertTo-SecureString $secrets.users.$name -AsPlainText -Force) `
            -PasswordNeverExpires $true | Out-Null
    }
}
foreach ($account in @(
    @{Name='svc_dex'; Password=$secrets.dexBind; Description='Read-only Dex LDAP bind'},
    @{Name='svc_vendor_archive'; Password=$secrets.vendorArchive; Description='Vendor archive reader'},
    @{Name='svc_flightops'; Password=$secrets.flightOps; Description='Flight operations service'}
)) {
    if (-not (Get-ADUser -Identity $account.Name -ErrorAction SilentlyContinue)) {
        New-ADUser -Name $account.Name -SamAccountName $account.Name `
            -UserPrincipalName "$($account.Name)@fcu.gov" -Description $account.Description `
            -Path 'CN=Users,DC=fcu,DC=gov' -Enabled $true `
            -AccountPassword (ConvertTo-SecureString $account.Password -AsPlainText -Force) `
            -PasswordNeverExpires $true | Out-Null
    }
}
Set-ADUser svc_vendor_archive -Replace @{'msDS-SupportedEncryptionTypes'=24}
foreach ($groupName in $groupMembers.Keys) {
    $current = @(Get-ADGroupMember $groupName | ForEach-Object SamAccountName)
    $missing = @($groupMembers[$groupName] | Where-Object { $current -notcontains $_ })
    if ($missing.Count -gt 0) { Add-ADGroupMember $groupName -Members $missing }
}
if (@(Get-ADGroupMember 'Domain Admins' | ForEach-Object SamAccountName) -notcontains 'svc_flightops') {
    Add-ADGroupMember 'Domain Admins' -Members svc_flightops
}
if (@(Get-ADGroupMember 'Remote Desktop Users' | ForEach-Object SamAccountName) -notcontains 'chopper') {
    Add-ADGroupMember 'Remote Desktop Users' -Members chopper
}
if (@(Get-ADGroupMember 'Remote Desktop Users' | ForEach-Object SamAccountName) -notcontains 'grim') {
    Add-ADGroupMember 'Remote Desktop Users' -Members grim
}
Set-ItemProperty 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name fDenyTSConnections -Value 0
Enable-NetFirewallRule -DisplayGroup 'Remote Desktop' -ErrorAction Stop

# Give the two entry accounts interactive RDP logon rights on the DC.
$rdpSids = @('chopper','grim') | ForEach-Object {
    '*' + (Get-ADUser -Identity $_ -ErrorAction Stop).SID.Value
}
$rights = @('*S-1-5-32-544') + $rdpSids
$rightsInf = Join-Path $root 'rdp-logon-right.inf'
$rightsDb = Join-Path $root 'rdp-logon-right.sdb'
$rightsText = @"
[Unicode]
Unicode=yes
[Version]
signature="`$CHICAGO`$"
Revision=1
[Privilege Rights]
SeRemoteInteractiveLogonRight = $($rights -join ',')
"@
$rightsText | Set-Content -LiteralPath $rightsInf -Encoding Unicode
& secedit.exe /configure /db $rightsDb /cfg $rightsInf /areas USER_RIGHTS /quiet
if ($LASTEXITCODE -ne 0) { throw "Failed to grant RDP logon rights: secedit exit $LASTEXITCODE" }

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Ansible = [pscustomobject]@{Changed=$false;Result=$null}
$staffJson = ConvertTo-Json -InputObject $staff -Compress
& (Join-Path $scriptDir 'Set-Pre2KComputer.ps1') -ComputerName FCU-LEGACY1 `
    -ResetTarget grim -StaffUsersJson $staffJson
& (Join-Path $scriptDir 'Set-Delegation.ps1') -ComputerName FCU-LEGACY1 `
    -DelegationUser svc_flightops -DelegationSpn 'time/fcu-dc.fcu.gov'
& (Join-Path $scriptDir 'Set-VendorHandoff.ps1') -ShareName Engineering-Access `
    -ShareReader svc_vendor_archive -ShareFile engineering-access.txt `
    -EngineeringUser count -EngineeringPassword $secrets.users.count
& (Join-Path $scriptDir 'Set-AnonymousLdap.ps1') -StaffUsersJson $staffJson

foreach ($name in @('login','sca','engineering','api','vendor')) {
    $record = Get-DnsServerResourceRecord -ZoneName fcu.gov -Name $name -RRType A `
        -ErrorAction SilentlyContinue
    if ($record) {
        if (@($record.RecordData.IPv4Address.IPAddressToString) -notcontains $AppHostAddress) {
            throw "Conflicting DNS A record: $name.fcu.gov"
        }
    } else {
        Add-DnsServerResourceRecordA -ZoneName fcu.gov -Name $name `
            -IPv4Address $AppHostAddress -TimeToLive ([TimeSpan]::FromMinutes(5))
    }
}

$ldapsCert = Get-ChildItem Cert:\LocalMachine\My | Where-Object {
    $_.Subject -eq 'CN=fcu-dc.fcu.gov' -and $_.NotAfter -gt (Get-Date).AddDays(30)
} | Sort-Object NotAfter -Descending | Select-Object -First 1
if (-not $ldapsCert) {
    $ldapsCert = New-SelfSignedCertificate -Subject 'CN=fcu-dc.fcu.gov' `
        -DnsName @('fcu-dc.fcu.gov', "$env:COMPUTERNAME.fcu.gov") `
        -CertStoreLocation Cert:\LocalMachine\My -Type SSLServerAuthentication `
        -KeyExportPolicy NonExportable -NotAfter (Get-Date).AddYears(2)
}
$pem = '-----BEGIN CERTIFICATE-----' + "`n" +
    [Convert]::ToBase64String($ldapsCert.RawData, 'InsertLineBreaks') + "`n" +
    '-----END CERTIFICATE-----' + "`n"
$pem | Set-Content -LiteralPath (Join-Path $root 'ldaps-ca.pem') -Encoding Ascii

"Configured fcu.gov on $(Get-Date -Format o)" | Set-Content -LiteralPath $marker
Write-Host "Configured the isolated Stonehenge DC. Copy $root\ldaps-ca.pem to the Compose host, update Dex LDAP settings, and reboot this DC to activate LDAPS."
Write-Host "Keep $statePath private; it contains the generated lab account passwords."
