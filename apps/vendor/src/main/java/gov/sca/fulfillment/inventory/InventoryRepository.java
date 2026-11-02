package gov.sca.fulfillment.inventory;

import jakarta.persistence.LockModeType;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.EntityGraph;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.Optional;

public interface InventoryRepository extends JpaRepository<Inventory, Long> {
    Optional<Inventory> findByPartId(Long partId);

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select i from Inventory i join fetch i.part where i.part.id = :partId")
    Optional<Inventory> lockByPartId(@Param("partId") Long partId);

    @Query("""
            select i from Inventory i join i.part p
            where (:q = '' or lower(p.sku) like concat('%', :q, '%')
                or lower(p.name) like concat('%', :q, '%')
                or lower(i.binLocation) like concat('%', :q, '%'))
            order by p.name asc
            """)
    @EntityGraph(attributePaths = {"part"})
    Page<Inventory> search(@Param("q") String q, Pageable pageable);
}
