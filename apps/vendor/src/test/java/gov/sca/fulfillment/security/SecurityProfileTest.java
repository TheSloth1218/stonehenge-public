package gov.sca.fulfillment.security;

import org.junit.jupiter.api.Test;
import org.springframework.context.annotation.Profile;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class SecurityProfileTest {
    @Test
    void oidcIsTheDefaultOutsideDevAndTest() {
        assertThat(SecurityConfiguration.class.getAnnotation(Profile.class).value())
                .containsExactly("!dev & !test");
        assertThat(DevelopmentSecurityConfiguration.class.getAnnotation(Profile.class).value())
                .containsExactly("dev", "test");
    }

    @Test
    void oidcConfigurationRequiresConfidentialClientSecret() {
        SecurityProperties security = new SecurityProperties(
                "SCA-Employees", "SCA-Admins", false);

        assertThatThrownBy(() -> new OidcConfigurationValidator(
                security, "stonehenge-vendor", "", "https://login.fcu.gov/dex",
                "https://vendor.fcu.gov", "a".repeat(64)))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("OIDC_CLIENT_SECRET");
    }

    @Test
    void oidcConfigurationRejectsOverlappingRoleGroups() {
        SecurityProperties security = new SecurityProperties("sca-users", "sca-users", false);

        assertThatThrownBy(() -> new OidcConfigurationValidator(
                security, "stonehenge-vendor", "secret", "https://login.fcu.gov/dex",
                "https://vendor.fcu.gov", "a".repeat(64)))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("must be different");
    }

    @Test
    void validOidcConfigurationIsAccepted() {
        SecurityProperties security = new SecurityProperties(
                "SCA-Employees", "SCA-Admins", false);

        new OidcConfigurationValidator(
                security, "stonehenge-vendor", "secret", "https://login.fcu.gov/dex",
                "https://vendor.fcu.gov", "a".repeat(64));
    }

    @Test
    void oidcConfigurationRequiresHttpsOrigins() {
        SecurityProperties security = new SecurityProperties(
                "SCA-Employees", "SCA-Admins", false);

        assertThatThrownBy(() -> new OidcConfigurationValidator(
                security, "stonehenge-vendor", "secret", "http://login.fcu.gov/dex",
                "https://vendor.fcu.gov", "a".repeat(64)))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("OIDC_ISSUER");
    }

    @Test
    void oidcConfigurationRequiresIndependentSessionSecret() {
        SecurityProperties security = new SecurityProperties(
                "SCA-Employees", "SCA-Admins", false);

        assertThatThrownBy(() -> new OidcConfigurationValidator(
                security, "stonehenge-vendor", "secret", "https://login.fcu.gov/dex",
                "https://vendor.fcu.gov", "too-short"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("AUTH_SESSION_SECRET");
    }
}
