package gov.sca.fulfillment.security;

import org.junit.jupiter.api.Test;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.authority.SimpleGrantedAuthority;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class AuthorizationServiceTest {
    private final UsernamePasswordAuthenticationToken employee = new UsernamePasswordAuthenticationToken(
            "employee", "N/A", List.of(new SimpleGrantedAuthority("ROLE_EMPLOYEE")));
    private final UsernamePasswordAuthenticationToken admin = new UsernamePasswordAuthenticationToken(
            "admin", "N/A", List.of(new SimpleGrantedAuthority("ROLE_EMPLOYEE"), new SimpleGrantedAuthority("ROLE_ADMIN")));

    @Test
    void safeModeRequiresAdminForSensitiveOperations() {
        AuthorizationService service = new AuthorizationService(new TrainingProperties(false));
        assertThat(service.adminOrTrainingEmployee(employee)).isFalse();
        assertThat(service.adminOrTrainingEmployee(admin)).isTrue();
    }

    @Test
    void trainingModeAllowsOnlyAuthenticatedEmployeeOrAdmin() {
        AuthorizationService service = new AuthorizationService(new TrainingProperties(true));
        assertThat(service.adminOrTrainingEmployee(employee)).isTrue();
        assertThat(service.adminOrTrainingEmployee(admin)).isTrue();
        assertThat(service.adminOrTrainingEmployee(null)).isFalse();
    }
}
