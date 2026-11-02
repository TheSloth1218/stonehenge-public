package gov.sca.fulfillment.security;

import gov.sca.fulfillment.users.AppUser;
import gov.sca.fulfillment.users.AppUserRepository;
import gov.sca.fulfillment.users.UserRole;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.security.oauth2.core.OAuth2AuthenticationException;
import org.springframework.security.oauth2.core.oidc.OidcIdToken;
import org.springframework.security.oauth2.core.oidc.user.DefaultOidcUser;
import org.springframework.security.oauth2.core.oidc.user.OidcUser;

import java.time.Instant;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class OidcUserMapperTest {
    private AppUserRepository users;
    private OidcUserMapper mapper;

    @BeforeEach
    void setUp() {
        users = mock(AppUserRepository.class);
        mapper = new OidcUserMapper(users,
                new SecurityProperties("SCA-Employees", "SCA-Admins", false));
        when(users.findByDexSubject("dex-subject")).thenReturn(Optional.empty());
        when(users.findByUsername("avery")).thenReturn(Optional.empty());
        when(users.save(any(AppUser.class))).thenAnswer(invocation -> invocation.getArgument(0));
    }

    @Test
    void mapsEmployeeGroupAndPersistsSubjectBoundProfile() {
        OidcUser mapped = mapper.mapUser(userWithGroups("SCA-Employees"));

        assertThat(mapped.getAuthorities()).extracting("authority").contains("ROLE_EMPLOYEE");
        AppUser saved = captureSavedUser();
        assertThat(saved.getDexSubject()).isEqualTo("dex-subject");
        assertThat(saved.getUsername()).isEqualTo("avery");
        assertThat(saved.getRole()).isEqualTo(UserRole.EMPLOYEE);
    }

    @Test
    void adminInheritsEmployeeAuthority() {
        OidcUser mapped = mapper.mapUser(userWithGroups("SCA-Admins"));

        assertThat(mapped.getAuthorities()).extracting("authority")
                .contains("ROLE_EMPLOYEE", "ROLE_ADMIN");
        assertThat(captureSavedUser().getRole()).isEqualTo(UserRole.ADMIN);
    }

    @Test
    void deniesIdentityWithoutTrustedGroup() {
        assertThatThrownBy(() -> mapper.mapUser(userWithGroups("some-other-group")))
                .isInstanceOf(OAuth2AuthenticationException.class)
                .satisfies(error -> assertThat(((OAuth2AuthenticationException) error).getError().getErrorCode())
                        .isEqualTo("access_denied"));
        verify(users, never()).save(any());
    }

    @Test
    void deniesIdentityWithOnlyEngineeringGroups() {
        assertThatThrownBy(() -> mapper.mapUser(
                userWithGroups("SCA-Engineering-Users", "SCA-Engineering-Admins")))
                .isInstanceOf(OAuth2AuthenticationException.class)
                .satisfies(error -> assertThat(((OAuth2AuthenticationException) error).getError().getErrorCode())
                        .isEqualTo("access_denied"));
        verify(users, never()).save(any());
    }

    @Test
    void ignoresEngineeringGroupsForVendorEmployee() {
        OidcUser mapped = mapper.mapUser(userWithGroups("SCA-Employees", "SCA-Engineering-Admins"));

        assertThat(mapped.getAuthorities()).extracting("authority")
                .contains("ROLE_EMPLOYEE")
                .doesNotContain("ROLE_ADMIN", "engineering.user", "engineering.admin");
        assertThat(captureSavedUser().getRole()).isEqualTo(UserRole.EMPLOYEE);
    }

    @Test
    void ignoresApplicationRolesOutsideSignedGroupsClaim() {
        assertThatThrownBy(() -> mapper.mapUser(userWithClaim("roles", "SCA-Admins")))
                .isInstanceOf(OAuth2AuthenticationException.class)
                .satisfies(error -> assertThat(((OAuth2AuthenticationException) error).getError().getErrorCode())
                        .isEqualTo("access_denied"));
        verify(users, never()).save(any());
    }

    private OidcUser userWithGroups(String... groups) {
        return userWithClaim("groups", groups);
    }

    private OidcUser userWithClaim(String claimName, String... values) {
        Instant now = Instant.now();
        OidcIdToken token = OidcIdToken.withTokenValue("test-token")
                .issuedAt(now)
                .expiresAt(now.plusSeconds(300))
                .subject("dex-subject")
                .claim("preferred_username", "avery")
                .claim("name", "Avery Stone")
                .claim("email", "avery@example.test")
                .claim(claimName, List.of(values))
                .build();
        return new DefaultOidcUser(List.of(), token, "sub");
    }

    private AppUser captureSavedUser() {
        var captor = org.mockito.ArgumentCaptor.forClass(AppUser.class);
        verify(users).save(captor.capture());
        return captor.getValue();
    }
}
