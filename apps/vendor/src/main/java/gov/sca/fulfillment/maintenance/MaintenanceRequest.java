package gov.sca.fulfillment.maintenance;

import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;
import java.time.Instant;

@Entity
@Table(name = "maintenance_requests")
public class MaintenanceRequest {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    private String externalRequestId;
    private String title;
    @Column(length = 4000)
    private String description;
    private String sourceAsset;
    private String sourcePanel;
    @Enumerated(EnumType.STRING)
    private Priority priority;
    private String remoteStatus;
    private boolean remoteAcknowledged;
    @Enumerated(EnumType.STRING)
    private LocalStatus localStatus;
    private Instant requestedAt;
    private Instant importedAt;
    private Instant lastSyncedAt;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "assigned_user_id")
    private AppUser assignedUser;
    @Column(columnDefinition = "jsonb")
    @JdbcTypeCode(SqlTypes.JSON)
    private String rawSnapshotJson;
    @Version
    private long version;

    public Long getId() { return id; }
    public String getExternalRequestId() { return externalRequestId; }
    public String getTitle() { return title; }
    public String getDescription() { return description; }
    public String getSourceAsset() { return sourceAsset; }
    public String getSourcePanel() { return sourcePanel; }
    public Priority getPriority() { return priority; }
    public String getRemoteStatus() { return remoteStatus; }
    public boolean isRemoteAcknowledged() { return remoteAcknowledged; }
    public LocalStatus getLocalStatus() { return localStatus; }
    public Instant getRequestedAt() { return requestedAt; }
    public Instant getImportedAt() { return importedAt; }
    public Instant getLastSyncedAt() { return lastSyncedAt; }
    public AppUser getAssignedUser() { return assignedUser; }
    public String getRawSnapshotJson() { return rawSnapshotJson; }
    public long getVersion() { return version; }
    public void setId(Long id) { this.id = id; }
    public void setExternalRequestId(String externalRequestId) { this.externalRequestId = externalRequestId; }
    public void setTitle(String title) { this.title = title; }
    public void setDescription(String description) { this.description = description; }
    public void setSourceAsset(String sourceAsset) { this.sourceAsset = sourceAsset; }
    public void setSourcePanel(String sourcePanel) { this.sourcePanel = sourcePanel; }
    public void setPriority(Priority priority) { this.priority = priority; }
    public void setRemoteStatus(String remoteStatus) { this.remoteStatus = remoteStatus; }
    public void setRemoteAcknowledged(boolean remoteAcknowledged) { this.remoteAcknowledged = remoteAcknowledged; }
    public void setLocalStatus(LocalStatus localStatus) { this.localStatus = localStatus; }
    public void setRequestedAt(Instant requestedAt) { this.requestedAt = requestedAt; }
    public void setImportedAt(Instant importedAt) { this.importedAt = importedAt; }
    public void setLastSyncedAt(Instant lastSyncedAt) { this.lastSyncedAt = lastSyncedAt; }
    public void setAssignedUser(AppUser assignedUser) { this.assignedUser = assignedUser; }
    public void setRawSnapshotJson(String rawSnapshotJson) { this.rawSnapshotJson = rawSnapshotJson; }
    public void setVersion(long version) { this.version = version; }
}
