package gov.sca.fulfillment.maintenance;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.Optional;

public interface MaintenanceRequestRepository extends JpaRepository<MaintenanceRequest, Long> {
    @EntityGraph(attributePaths = {"assignedUser"})
    Optional<MaintenanceRequest> findWithAssignedUserById(Long id);

    Optional<MaintenanceRequest> findByExternalRequestId(String externalRequestId);

    @Query("""
            select r from MaintenanceRequest r
            where (:q = '' or lower(r.externalRequestId) like concat('%', :q, '%')
                or lower(r.title) like concat('%', :q, '%')
                or lower(r.sourceAsset) like concat('%', :q, '%'))
              and (:status is null or r.localStatus = :status)
              and (:priority is null or r.priority = :priority)
            order by r.importedAt desc
            """)
    @EntityGraph(attributePaths = {"assignedUser"})
    Page<MaintenanceRequest> search(@Param("q") String q,
                                    @Param("status") LocalStatus status,
                                    @Param("priority") Priority priority,
                                    Pageable pageable);
}
