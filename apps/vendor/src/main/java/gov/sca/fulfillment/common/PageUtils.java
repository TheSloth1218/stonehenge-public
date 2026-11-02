package gov.sca.fulfillment.common;

import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Pageable;
import org.springframework.data.domain.Sort;

public final class PageUtils {
    private PageUtils() {
    }

    public static Pageable page(int page, int size, String sortProperty) {
        int safePage = Math.max(page, 0);
        int safeSize = Math.min(Math.max(size, 5), 50);
        return PageRequest.of(safePage, safeSize, Sort.by(sortProperty).descending());
    }

    public static String blankToNull(String value) {
        return value == null || value.isBlank() ? null : value.trim();
    }

    public static String searchTerm(String value) {
        return value == null || value.isBlank() ? "" : value.trim().toLowerCase();
    }
}
