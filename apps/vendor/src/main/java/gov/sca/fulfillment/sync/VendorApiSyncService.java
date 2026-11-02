package gov.sca.fulfillment.sync;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import gov.sca.fulfillment.maintenance.LocalStatus;
import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import gov.sca.fulfillment.maintenance.MaintenanceRequestRepository;
import gov.sca.fulfillment.maintenance.Priority;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.Instant;
import java.util.Comparator;
import java.util.List;
import java.util.Locale;
import java.util.Set;
import java.util.stream.Collectors;

@Service
public class VendorApiSyncService {
    private static final Logger log = LoggerFactory.getLogger(VendorApiSyncService.class);
    private final StonehengeApiClient api;
    private final StonehengeApiProperties properties;
    private final MaintenanceRequestRepository requests;
    private final ObjectMapper objectMapper;
    private final TransactionTemplate transactions;
    private volatile List<VendorGunStatusDto> gunStatuses = List.of();
    private volatile List<VendorCompletionDto> completions = List.of();
    private volatile VendorApiSyncState state = new VendorApiSyncState(false, null, "Waiting for Stonehenge API synchronization");

    public VendorApiSyncService(StonehengeApiClient api, StonehengeApiProperties properties,
                                MaintenanceRequestRepository requests, ObjectMapper objectMapper,
                                TransactionTemplate transactions) {
        this.api = api;
        this.properties = properties;
        this.requests = requests;
        this.objectMapper = objectMapper;
        this.transactions = transactions;
    }

    @EventListener(ApplicationReadyEvent.class)
    public void initialSync() {
        if (properties.pollingEnabled()) {
            refreshQuietly();
        }
    }

    @Scheduled(fixedDelayString = "${stonehenge.api.poll-interval-ms:30000}", initialDelayString = "${stonehenge.api.poll-interval-ms:30000}")
    public void scheduledSync() {
        if (properties.pollingEnabled()) {
            refreshQuietly();
        }
    }

    public VendorApiSyncState refreshNow() {
        int attempts = Math.max(1, properties.pollingAttempts());
        StonehengeApiException failure = null;
        for (int attempt = 1; attempt <= attempts; attempt++) {
            try {
                List<VendorPartRequestDto> remoteRequests = api.getPartRequests();
                List<VendorCompletionDto> remoteCompletions = api.getVendorCompletions();
                List<VendorGunStatusDto> remoteGuns = api.getGunStatus();
                List<VendorPartRequestDto> preparedRequests = checkedCopy(remoteRequests, "vendor part requests");
                List<VendorCompletionDto> preparedCompletions = checkedCopy(remoteCompletions, "vendor completions");
                List<VendorGunStatusDto> preparedGuns = checkedCopy(remoteGuns, "vendor gun status").stream()
                        .sorted(Comparator.comparingInt(VendorGunStatusDto::gunId))
                        .toList();
                Instant now = Instant.now();
                transactions.executeWithoutResult(status -> apply(preparedRequests, preparedCompletions, now));
                completions = preparedCompletions;
                gunStatuses = preparedGuns;
                state = new VendorApiSyncState(true, now, "Connected");
                return state;
            } catch (StonehengeApiException ex) {
                failure = ex;
                if (attempt < attempts) {
                    backoff(attempt);
                }
            }
        }
        String message = failure == null ? "Stonehenge API synchronization failed." : failure.getMessage();
        state = new VendorApiSyncState(false, state.lastSuccessfulSync(), message);
        throw new StonehengeApiException(message, failure);
    }

    public VendorAcknowledgeDto acknowledge(String requestId) {
        VendorAcknowledgeDto response = api.acknowledgePartRequest(requestId);
        transactions.executeWithoutResult(status -> requests.findByExternalRequestId(requestId).ifPresent(request -> {
            request.setRemoteAcknowledged(response.acknowledged());
            request.setRemoteStatus(response.vendorState());
            request.setLastSyncedAt(Instant.now());
            if (request.getLocalStatus() == LocalStatus.NEW) {
                request.setLocalStatus(LocalStatus.IN_PROGRESS);
            }
        }));
        return response;
    }

    public List<VendorGunStatusDto> gunStatuses() {
        return gunStatuses;
    }

    public List<VendorCompletionDto> completions() {
        return completions;
    }

    public VendorApiSyncState state() {
        return state;
    }

    private void refreshQuietly() {
        try {
            refreshNow();
        } catch (StonehengeApiException ex) {
            log.warn("Stonehenge vendor API synchronization is unavailable; local workflow remains available");
        }
    }

    private void apply(List<VendorPartRequestDto> remoteRequests, List<VendorCompletionDto> remoteCompletions, Instant now) {
        Set<String> completedIds = remoteCompletions.stream()
                .map(VendorCompletionDto::requestId)
                .filter(id -> id != null && !id.isBlank())
                .collect(Collectors.toSet());
        for (VendorPartRequestDto remote : remoteRequests) {
            if (remote.requestId() == null || remote.requestId().isBlank()) {
                continue;
            }
            MaintenanceRequest local = requests.findByExternalRequestId(remote.requestId()).orElseGet(MaintenanceRequest::new);
            boolean created = local.getId() == null;
            local.setExternalRequestId(remote.requestId());
            local.setTitle("Gun " + remote.gunId() + " parts/service request");
            local.setDescription("Engineering part status: " + display(remote.partStatus()) + ".");
            local.setSourceAsset("Gun " + remote.gunId());
            local.setSourcePanel(null);
            local.setPriority(priority(remote.priority()));
            local.setRemoteStatus(remote.status());
            local.setRemoteAcknowledged(remote.acknowledged());
            local.setRequestedAt(remote.approvedAt() == null ? now : remote.approvedAt());
            if (created) {
                local.setImportedAt(now);
                local.setLocalStatus(remote.acknowledged() ? LocalStatus.IN_PROGRESS : LocalStatus.NEW);
            }
            if (completedIds.contains(remote.requestId()) && local.getLocalStatus() != LocalStatus.SYNC_FAILED) {
                local.setLocalStatus(LocalStatus.FULFILLED);
            }
            local.setLastSyncedAt(now);
            local.setRawSnapshotJson(snapshot(remote));
            requests.save(local);
        }
        for (VendorCompletionDto completion : remoteCompletions) {
            if (completion.requestId() != null) {
                requests.findByExternalRequestId(completion.requestId()).ifPresent(local -> {
                    local.setRemoteStatus(completion.vendorState());
                    local.setRemoteAcknowledged(true);
                    local.setLastSyncedAt(now);
                    if (local.getLocalStatus() != LocalStatus.SYNC_FAILED) {
                        local.setLocalStatus(LocalStatus.FULFILLED);
                    }
                });
            }
        }
    }

    private String snapshot(VendorPartRequestDto remote) {
        try {
            return objectMapper.writeValueAsString(remote);
        } catch (JsonProcessingException ex) {
            return "{}";
        }
    }

    private Priority priority(String value) {
        if (value == null) return Priority.NORMAL;
        try {
            return Priority.valueOf(value.trim().toUpperCase(Locale.ROOT));
        } catch (IllegalArgumentException ex) {
            return Priority.NORMAL;
        }
    }

    private String display(String value) {
        if (value == null || value.isBlank()) return "not specified";
        return value.replaceAll("([a-z])([A-Z])", "$1 $2").toLowerCase(Locale.ROOT);
    }

    private <T> List<T> checkedCopy(List<T> values, String responseName) {
        try {
            return List.copyOf(values);
        } catch (NullPointerException ex) {
            throw new StonehengeApiException("Stonehenge API returned an invalid " + responseName + " response.", ex);
        }
    }

    private void backoff(int attempt) {
        try {
            Thread.sleep(250L * (1L << Math.min(attempt - 1, 3)));
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new StonehengeApiException("Stonehenge API synchronization was interrupted.", ex);
        }
    }
}
