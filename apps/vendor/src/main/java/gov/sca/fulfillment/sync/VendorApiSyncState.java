package gov.sca.fulfillment.sync;

import java.time.Instant;

public record VendorApiSyncState(boolean available, Instant lastSuccessfulSync, String message) {
}
