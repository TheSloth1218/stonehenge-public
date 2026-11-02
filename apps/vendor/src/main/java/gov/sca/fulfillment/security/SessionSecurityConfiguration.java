package gov.sca.fulfillment.security;

import org.apache.catalina.Manager;
import org.apache.catalina.session.StandardManager;
import org.apache.catalina.util.StandardSessionIdGenerator;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.web.embedded.tomcat.TomcatServletWebServerFactory;
import org.springframework.boot.web.server.WebServerFactoryCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.SecureRandom;
import java.util.Base64;

@Configuration
@Profile("!dev & !test")
class SessionSecurityConfiguration {
    @Bean
    WebServerFactoryCustomizer<TomcatServletWebServerFactory> keyedSessionIds(
            @Value("${app.auth-session-secret}") String secret) {
        return factory -> factory.addContextCustomizers(context -> {
            Manager manager = context.getManager();
            if (manager == null) {
                manager = new StandardManager();
                context.setManager(manager);
            }
            manager.setSessionIdGenerator(new HmacSessionIdGenerator(secret));
        });
    }

    static final class HmacSessionIdGenerator extends StandardSessionIdGenerator {
        private final SecureRandom random = new SecureRandom();
        private final SecretKeySpec key;

        HmacSessionIdGenerator(String secret) {
            this.key = new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256");
        }

        @Override
        public String generateSessionId(String route) {
            byte[] nonce = new byte[32];
            random.nextBytes(nonce);
            try {
                Mac mac = Mac.getInstance("HmacSHA256");
                mac.init(key);
                String id = Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(nonce));
                return route == null || route.isBlank() ? id : id + "." + route;
            } catch (GeneralSecurityException error) {
                throw new IllegalStateException("Unable to generate an authenticated session identifier", error);
            }
        }
    }
}
