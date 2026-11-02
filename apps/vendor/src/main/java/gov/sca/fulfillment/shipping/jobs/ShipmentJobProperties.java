package gov.sca.fulfillment.shipping.jobs;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "sca.shipment")
public record ShipmentJobProperties(
        Compatibility compatibility,
        Worker worker,
        String workDirectory
) {
    public record Compatibility(boolean enabled, String format, String exportRoute, String replayRoute) {}
    public record Worker(boolean enabled, String interval, int maxAttempts) {}
}
