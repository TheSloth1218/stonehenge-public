package gov.sca.fulfillment.catalog;

import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.List;
import java.util.Optional;

public interface PartRepository extends JpaRepository<Part, Long> {
    @Query("""
            select p from Part p
            where (:q = '' or lower(p.sku) like concat('%', lower(:q), '%')
                or lower(p.name) like concat('%', lower(:q), '%')
                or lower(p.description) like concat('%', lower(:q), '%'))
              and (:category is null or p.category = :category)
              and (:active is null or p.active = :active)
            order by p.name asc
            """)
    Page<Part> search(@Param("q") String q,
                      @Param("category") String category,
                      @Param("active") Boolean active,
                      Pageable pageable);

    @Query("select distinct p.category from Part p order by p.category")
    List<String> categories();

    Optional<Part> findBySku(String sku);

    @Query("select count(i) from FulfillmentOrderItem i where i.part.id = :partId")
    long historicalReferenceCount(@Param("partId") Long partId);
}
