package gov.sca.fulfillment.security;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.security.config.annotation.method.configuration.EnableMethodSecurity;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.oauth2.client.registration.ClientRegistrationRepository;
import org.springframework.security.oauth2.client.web.DefaultOAuth2AuthorizationRequestResolver;
import org.springframework.security.oauth2.client.web.OAuth2AuthorizationRequestCustomizers;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.LoginUrlAuthenticationEntryPoint;

@Configuration
@Profile("!dev & !test")
@EnableMethodSecurity
public class SecurityConfiguration {
    @Bean
    SecurityFilterChain oidcSecurity(HttpSecurity http,
                                     OidcUserMapper users,
                                     OidcAuthenticationFailureHandler failures,
                                     TrainingProperties training,
                                     ClientRegistrationRepository clients) throws Exception {
        SecurityRules.authorize(http, training.enabled());
        http.oauth2Login(login -> login
                .authorizationEndpoint(endpoint -> endpoint
                        .authorizationRequestResolver(pkceResolver(clients)))
                .redirectionEndpoint(endpoint -> endpoint.baseUri("/login/oauth2/code/*"))
                .failureHandler(failures)
                .userInfoEndpoint(info -> info.oidcUserService(users)));
        http.exceptionHandling(exceptions -> exceptions
                .authenticationEntryPoint(new LoginUrlAuthenticationEntryPoint("/oauth2/authorization/dex")));
        return http.build();
    }

    static DefaultOAuth2AuthorizationRequestResolver pkceResolver(ClientRegistrationRepository clients) {
        DefaultOAuth2AuthorizationRequestResolver resolver =
                new DefaultOAuth2AuthorizationRequestResolver(clients, "/oauth2/authorization");
        resolver.setAuthorizationRequestCustomizer(OAuth2AuthorizationRequestCustomizers.withPkce());
        return resolver;
    }
}
