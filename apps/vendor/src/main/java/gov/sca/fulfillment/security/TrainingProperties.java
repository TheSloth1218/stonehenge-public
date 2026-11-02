package gov.sca.fulfillment.security;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "sca.training")
public record TrainingProperties(boolean enabled) {
}
