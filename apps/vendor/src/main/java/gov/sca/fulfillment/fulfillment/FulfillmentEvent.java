package gov.sca.fulfillment.fulfillment;

import gov.sca.fulfillment.maintenance.MaintenanceRequest;
import gov.sca.fulfillment.users.AppUser;
import jakarta.persistence.*;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;
import java.time.Instant;

@Entity
@Table(name = "fulfillment_events")
public class FulfillmentEvent {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "maintenance_request_id")
    private MaintenanceRequest maintenanceRequest;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "fulfillment_order_id")
    private FulfillmentOrder fulfillmentOrder;
    @Enumerated(EnumType.STRING)
    private EventType eventType;
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "actor_user_id")
    private AppUser actorUser;
    private String summary;
    @Column(columnDefinition = "jsonb")
    @JdbcTypeCode(SqlTypes.JSON)
    private String detailsJson;
    private Instant createdAt;

    public Long getId() { return id; }
    public MaintenanceRequest getMaintenanceRequest() { return maintenanceRequest; }
    public FulfillmentOrder getFulfillmentOrder() { return fulfillmentOrder; }
    public EventType getEventType() { return eventType; }
    public String getSummary() { return summary; }
    public Instant getCreatedAt() { return createdAt; }
    public void setMaintenanceRequest(MaintenanceRequest maintenanceRequest) { this.maintenanceRequest = maintenanceRequest; }
    public void setFulfillmentOrder(FulfillmentOrder fulfillmentOrder) { this.fulfillmentOrder = fulfillmentOrder; }
    public void setEventType(EventType eventType) { this.eventType = eventType; }
    public void setActorUser(AppUser actorUser) { this.actorUser = actorUser; }
    public void setSummary(String summary) { this.summary = summary; }
    public void setDetailsJson(String detailsJson) { this.detailsJson = detailsJson; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
}
