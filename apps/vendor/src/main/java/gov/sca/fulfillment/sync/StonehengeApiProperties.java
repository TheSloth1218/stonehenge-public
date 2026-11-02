package gov.sca.fulfillment.sync;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.time.Duration;

@ConfigurationProperties(prefix = "stonehenge.api")
public record StonehengeApiProperties(
        String baseUrl,
        String apiKey,
        Duration connectTimeout,
        Duration readTimeout,
        boolean pollingEnabled,
        int pollingAttempts
) {
}
