package gov.sca.fulfillment.security;

import gov.sca.fulfillment.users.AppUserRepository;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.security.config.annotation.method.configuration.EnableMethodSecurity;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.AnonymousAuthenticationFilter;

@Configuration
@Profile({"dev", "test"})
@EnableMethodSecurity
public class DevelopmentSecurityConfiguration {
    @Bean
    SecurityFilterChain developmentSecurity(HttpSecurity http,
                                             AppUserRepository users,
                                             TrainingProperties training,
                                             @Value("${app.dev-user}") String username) throws Exception {
        SecurityRules.authorize(http, training.enabled());
        http.addFilterBefore(new DevAuthenticationFilter(users, username), AnonymousAuthenticationFilter.class);
        return http.build();
    }
}
