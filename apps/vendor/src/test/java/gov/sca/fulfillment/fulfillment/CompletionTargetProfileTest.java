package gov.sca.fulfillment.fulfillment;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class CompletionTargetProfileTest {
    @Test
    void trainingUsesSubmittedExternalRequestId() {
        assertThat(FulfillmentService.completionTarget(true, "persisted-2001", " 2003 ")).isEqualTo("2003");
    }

    @Test
    void safeAlwaysUsesPersistedRelationship() {
        assertThat(FulfillmentService.completionTarget(false, "persisted-2001", "2003")).isEqualTo("persisted-2001");
    }
}
