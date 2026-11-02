package gov.sca.fulfillment.security;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class SessionSecurityConfigurationTest {
    @Test
    void sessionIdentifiersAreOpaqueUniqueAndRouteAware() {
        var generator = new SessionSecurityConfiguration.HmacSessionIdGenerator("a".repeat(64));

        String first = generator.generateSessionId(null);
        String second = generator.generateSessionId("vendor");

        assertThat(first).hasSize(43).doesNotContain("aaaa");
        assertThat(second).endsWith(".vendor").doesNotStartWith(first);
    }
}
