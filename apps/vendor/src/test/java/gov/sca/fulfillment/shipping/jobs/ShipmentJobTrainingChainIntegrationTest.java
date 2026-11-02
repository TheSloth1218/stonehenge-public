package gov.sca.fulfillment.shipping.jobs;

import gov.sca.fulfillment.common.PostgresIntegrationTest;
import gov.sca.fulfillment.users.AppUserRepository;
import gov.sca.fulfillment.users.CurrentUserService;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.security.test.context.support.WithMockUser;

import java.nio.file.Files;
import java.nio.file.Path;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.when;

@SpringBootTest(properties = {
        "sca.training.enabled=true",
        "sca.shipment.worker.enabled=true",
        "sca.shipment.worker.interval=3600000"
})
class ShipmentJobTrainingChainIntegrationTest extends PostgresIntegrationTest {
    @Autowired ShipmentJobService service;
    @Autowired ShipmentJobWorker worker;
    @Autowired ShipmentJobRepository jobs;
    @Autowired AppUserRepository users;
    @MockBean CurrentUserService currentUser;
    private Path marker;

    @BeforeEach
    void setUp() throws Exception {
        jobs.deleteAll();
        when(currentUser.currentUser()).thenReturn(users.findByUsername("operator").orElseThrow());
        marker = Files.createTempFile("stonehenge-worker-cc6-", ".marker");
        Files.delete(marker);
    }

    @AfterEach
    void cleanUp() throws Exception {
        jobs.deleteAll();
        Files.deleteIfExists(marker);
    }

    @Test
    @WithMockUser(username = "operator", roles = "EMPLOYEE")
    void employeeReplayReachesTrainingWorkerAndExecutesOnce() throws Exception {
        ShipmentJob queued = service.replay(CommonsCollections6PayloadFactory.encode(
                "/usr/bin/touch " + marker.toAbsolutePath()));

        worker.poll();

        ShipmentJob completed = jobs.findById(queued.getId()).orElseThrow();
        assertThat(completed.getStatus()).isEqualTo(ShipmentJobStatus.COMPLETED);
        assertThat(completed.getAttemptCount()).isEqualTo(1);
        assertThat(marker).exists();
    }
}
