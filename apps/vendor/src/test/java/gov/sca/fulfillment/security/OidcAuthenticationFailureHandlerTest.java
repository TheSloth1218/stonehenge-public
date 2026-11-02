package gov.sca.fulfillment.security;

import org.junit.jupiter.api.Test;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.security.oauth2.core.OAuth2AuthenticationException;
import org.springframework.security.oauth2.core.OAuth2Error;

import static org.assertj.core.api.Assertions.assertThat;

class OidcAuthenticationFailureHandlerTest {
    @Test
    void callbackFailureRedirectsToStableDeniedPage() throws Exception {
        MockHttpServletResponse response = new MockHttpServletResponse();

        new OidcAuthenticationFailureHandler().onAuthenticationFailure(
                new MockHttpServletRequest(),
                response,
                new OAuth2AuthenticationException(
                        new OAuth2Error("access_denied", "No recognized group", null)));

        assertThat(response.getRedirectedUrl())
                .startsWith("/access-denied?reference=")
                .endsWith("&reason=oidc");
    }
}
