package gov.sca.fulfillment.sync;

import com.fasterxml.jackson.databind.ObjectMapper;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class VendorApiSyncServiceTest {
    @Test
    void rejectsMalformedResponsesBeforeStartingTheImportTransaction() {
        StonehengeApiClient api = mock(StonehengeApiClient.class);
        TransactionTemplate transactions = mock(TransactionTemplate.class);
        when(api.getPartRequests()).thenReturn(List.of());
        when(api.getVendorCompletions()).thenReturn(List.of());
        List<VendorGunStatusDto> malformedGunStatus = new ArrayList<>();
        // Two entries force the previous implementation's comparator to run.
        malformedGunStatus.add(new VendorGunStatusDto(1, "AUTO", 0, "Ready", null, Map.of()));
        malformedGunStatus.add(null);
        when(api.getGunStatus()).thenReturn(malformedGunStatus);

        VendorApiSyncService service = new VendorApiSyncService(
                api,
                new StonehengeApiProperties(
                        "http://stonehenge.test/api/v1",
                        "test-vendor-key",
                        Duration.ofSeconds(1),
                        Duration.ofSeconds(1),
                        false,
                        1),
                mock(MaintenanceRequestRepository.class),
                new ObjectMapper(),
                transactions);

        assertThatThrownBy(service::refreshNow)
                .isInstanceOf(StonehengeApiException.class)
                .hasMessageContaining("invalid vendor gun status response");
        verify(transactions, never()).executeWithoutResult(org.mockito.ArgumentMatchers.any());
    }
}
