package gov.sca.fulfillment.common;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class AccessControllerTest {
    @Test
    void signedOutPageIsStableAndDoesNotTriggerLogin() {
        assertThat(new AccessController().signedOut()).isEqualTo("signed-out");
    }
}
