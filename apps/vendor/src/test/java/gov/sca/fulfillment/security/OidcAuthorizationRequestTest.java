package gov.sca.fulfillment.security;

import org.junit.jupiter.api.Test;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.security.oauth2.client.registration.ClientRegistration;
import org.springframework.security.oauth2.client.registration.InMemoryClientRegistrationRepository;
import org.springframework.security.oauth2.core.AuthorizationGrantType;
import org.springframework.security.oauth2.core.ClientAuthenticationMethod;

import static org.assertj.core.api.Assertions.assertThat;

class OidcAuthorizationRequestTest {
    @Test
    void authorizationCodeFlowUsesPkceS256AndExactCallback() {
        ClientRegistration dex = ClientRegistration.withRegistrationId("dex")
                .clientId("stonehenge-vendor")
                .clientSecret("test-secret")
                .clientAuthenticationMethod(ClientAuthenticationMethod.CLIENT_SECRET_BASIC)
                .authorizationGrantType(AuthorizationGrantType.AUTHORIZATION_CODE)
                .redirectUri("https://vendor.fcu.gov/login/oauth2/code/dex")
                .scope("openid", "profile", "email", "groups")
                .authorizationUri("https://login.fcu.gov/dex/auth")
                .tokenUri("https://login.fcu.gov/dex/token")
                .jwkSetUri("https://login.fcu.gov/dex/keys")
                .issuerUri("https://login.fcu.gov/dex")
                .userNameAttributeName("sub")
                .clientName("Stonehenge Vendor Portal")
                .build();
        var resolver = SecurityConfiguration.pkceResolver(
                new InMemoryClientRegistrationRepository(dex));
        MockHttpServletRequest request = new MockHttpServletRequest(
                "GET", "/oauth2/authorization/dex");
        request.setServletPath("/oauth2/authorization/dex");

        var authorization = resolver.resolve(request);

        assertThat(authorization).isNotNull();
        assertThat(authorization.getRedirectUri())
                .isEqualTo("https://vendor.fcu.gov/login/oauth2/code/dex");
        assertThat(authorization.getScopes())
                .containsExactlyInAnyOrder("openid", "profile", "email", "groups");
        assertThat(authorization.getAdditionalParameters())
                .containsEntry("code_challenge_method", "S256")
                .containsKey("code_challenge");
    }
}
