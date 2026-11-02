# Extracted from the reviewed Stonehenge directory playbook.
param([string]$StaffUsersJson)

Import-Module ActiveDirectory -ErrorAction Stop
$staffUsers = $StaffUsersJson | ConvertFrom-Json -ErrorAction Stop
$rootDse = Get-ADRootDSE -ErrorAction Stop
$anonymousSid = [Security.Principal.SecurityIdentifier]::new('S-1-5-7')
$emptyGuid = [Guid]::Empty

function Get-StonehengeSchemaGuid {
    param([string]$LdapDisplayName)
    $schemaObject = Get-ADObject -SearchBase $rootDse.schemaNamingContext `
        -LDAPFilter "(lDAPDisplayName=$LdapDisplayName)" `
        -Properties schemaIDGUID -ErrorAction Stop
    return [Guid]::new([byte[]]$schemaObject.schemaIDGUID)
}

function Test-StonehengeAccessRule {
    param(
        [System.DirectoryServices.ActiveDirectorySecurity]$Acl,
        [DirectoryServices.ActiveDirectoryRights]$Rights,
        [Guid]$ObjectType,
        [DirectoryServices.ActiveDirectorySecurityInheritance]$InheritanceType,
        [Guid]$InheritedObjectType
    )
    foreach ($ace in @($Acl.Access)) {
        try {
            $aceSid = $ace.IdentityReference.Translate(
                [Security.Principal.SecurityIdentifier]
            )
        }
        catch { continue }
        if (
            $aceSid -eq $anonymousSid -and
            -not $ace.IsInherited -and
            $ace.AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and
            ($ace.ActiveDirectoryRights -band $Rights) -eq $Rights -and
            $ace.ObjectType -eq $ObjectType -and
            $ace.InheritanceType -eq $InheritanceType -and
            $ace.InheritedObjectType -eq $InheritedObjectType
        ) {
            return $true
        }
    }
    return $false
}

$userClassGuid = Get-StonehengeSchemaGuid -LdapDisplayName user
$accountNameGuid = Get-StonehengeSchemaGuid -LdapDisplayName sAMAccountName
$descriptionGuid = Get-StonehengeSchemaGuid -LdapDisplayName description
$usersDn = "CN=Users,$($rootDse.defaultNamingContext)"
$changed = $false

$directoryServiceDn = `
    "CN=Directory Service,CN=Windows NT,CN=Services,$($rootDse.configurationNamingContext)"
$directoryService = Get-ADObject -Identity $directoryServiceDn `
    -Properties dSHeuristics -ErrorAction Stop
$heuristics = [string]$directoryService.dSHeuristics
if ($heuristics.Length -lt 7) {
    $heuristics = $heuristics.PadRight(7, '0')
}
$heuristicCharacters = $heuristics.ToCharArray()
$heuristicCharacters[2] = '1'
$heuristicCharacters[6] = '2'
$requiredHeuristics = -join $heuristicCharacters
if ($heuristics -ne $requiredHeuristics) {
    Set-ADObject -Identity $directoryServiceDn `
        -Replace @{dSHeuristics = $requiredHeuristics} -ErrorAction Stop
    $changed = $true
}

$usersAclPath = "AD:\$usersDn"
$usersAcl = Get-Acl -Path $usersAclPath -ErrorAction Stop
$usersAclChanged = $false
foreach ($staleRule in @($usersAcl.Access)) {
    try {
        $staleSid = $staleRule.IdentityReference.Translate(
            [Security.Principal.SecurityIdentifier]
        )
    }
    catch { continue }
    $isLegacyListChildren = (
        $staleRule.ActiveDirectoryRights -eq `
            [DirectoryServices.ActiveDirectoryRights]::ListChildren -and
        $staleRule.ObjectType -eq $userClassGuid
    )
    if (
        $staleSid -eq $anonymousSid -and
        -not $staleRule.IsInherited -and
        $staleRule.AccessControlType -eq `
            [Security.AccessControl.AccessControlType]::Allow -and
        $staleRule.InheritanceType -eq `
            [DirectoryServices.ActiveDirectorySecurityInheritance]::None -and
        $isLegacyListChildren
    ) {
        $usersAcl.RemoveAccessRuleSpecific($staleRule)
        $usersAclChanged = $true
        $changed = $true
    }
}
# Root-subtree searches must traverse CN=Users before object-specific
# ListObject and attribute rights can bound the visible staff entries.
if (-not (Test-StonehengeAccessRule -Acl $usersAcl `
    -Rights ([DirectoryServices.ActiveDirectoryRights]::ListObject) `
    -ObjectType $emptyGuid `
    -InheritanceType ([DirectoryServices.ActiveDirectorySecurityInheritance]::None) `
    -InheritedObjectType $emptyGuid)) {
    $usersAcl.AddAccessRule(
        [DirectoryServices.ActiveDirectoryAccessRule]::new(
            $anonymousSid,
            [DirectoryServices.ActiveDirectoryRights]::ListObject,
            [Security.AccessControl.AccessControlType]::Allow
        )
    )
    $usersAclChanged = $true
    $changed = $true
}
if ($usersAclChanged) {
    Set-Acl -Path $usersAclPath -AclObject $usersAcl -ErrorAction Stop
}

foreach ($staffUser in $staffUsers) {
    $user = Get-ADUser -Identity $staffUser.username -ErrorAction Stop
    $userAclPath = "AD:\$($user.DistinguishedName)"
    $userAcl = Get-Acl -Path $userAclPath -ErrorAction Stop
    $userAclChanged = $false
    if (-not (Test-StonehengeAccessRule -Acl $userAcl `
        -Rights ([DirectoryServices.ActiveDirectoryRights]::ListObject) `
        -ObjectType $emptyGuid `
        -InheritanceType ([DirectoryServices.ActiveDirectorySecurityInheritance]::None) `
        -InheritedObjectType $emptyGuid)) {
        $userAcl.AddAccessRule(
            [DirectoryServices.ActiveDirectoryAccessRule]::new(
                $anonymousSid,
                [DirectoryServices.ActiveDirectoryRights]::ListObject,
                [Security.AccessControl.AccessControlType]::Allow
            )
        )
        $userAclChanged = $true
    }
    foreach ($attributeGuid in @($accountNameGuid, $descriptionGuid)) {
        if (-not (Test-StonehengeAccessRule -Acl $userAcl `
            -Rights ([DirectoryServices.ActiveDirectoryRights]::ReadProperty) `
            -ObjectType $attributeGuid `
            -InheritanceType ([DirectoryServices.ActiveDirectorySecurityInheritance]::None) `
            -InheritedObjectType $emptyGuid)) {
            $userAcl.AddAccessRule(
                [DirectoryServices.ActiveDirectoryAccessRule]::new(
                    $anonymousSid,
                    [DirectoryServices.ActiveDirectoryRights]::ReadProperty,
                    [Security.AccessControl.AccessControlType]::Allow,
                    $attributeGuid,
                    [DirectoryServices.ActiveDirectorySecurityInheritance]::None
                )
            )
            $userAclChanged = $true
        }
    }
    if ($userAclChanged) {
        Set-Acl -Path $userAclPath -AclObject $userAcl -ErrorAction Stop
        $changed = $true
    }
}

$verifiedHeuristics = [string](Get-ADObject -Identity $directoryServiceDn `
    -Properties dSHeuristics -ErrorAction Stop).dSHeuristics
if (
    $verifiedHeuristics.Length -lt 7 -or
    $verifiedHeuristics[2] -ne '1' -or
    $verifiedHeuristics[6] -ne '2'
) {
    throw 'Anonymous LDAP operations are not enabled.'
}

$Ansible.Changed = $changed
$Ansible.Result = [ordered]@{
    StaffCount = @($staffUsers).Count
    Attributes = @('sAMAccountName', 'description')
    SearchBase = $rootDse.defaultNamingContext
}
