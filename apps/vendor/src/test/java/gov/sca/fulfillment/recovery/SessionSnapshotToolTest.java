package gov.sca.fulfillment.recovery;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class SessionSnapshotToolTest {
    @TempDir Path temporary;

    @Test
    void authenticatedEncryptionRoundTripsMultipleSelectableRecordsWithoutMetadataSecrets() throws Exception {
        Path key = temporary.resolve("key");
        Path encrypted = temporary.resolve("session-snapshot.db.enc");
        Path metadata = temporary.resolve("session-snapshot.meta");
        SessionSnapshotTool.keygen(key);
        SessionSnapshotTool.Snapshot source = validSnapshot();

        SessionSnapshotTool.writeSnapshot(source, encrypted, metadata, key);
        SessionSnapshotTool.Snapshot restored = SessionSnapshotTool.readSnapshot(encrypted, key);

        assertThat(restored.records).extracting(r -> r.id)
                .containsExactly("employee", "edge", "target", "revoked");
        String safeMetadata = Files.readString(metadata);
        assertThat(safeMetadata).contains("SCA-VENDOR-009", "target").doesNotContain("dex-artifact");
        assertThat(new String(Files.readAllBytes(encrypted), java.nio.charset.StandardCharsets.ISO_8859_1))
                .doesNotContain("swordsman");
    }

    @Test
    void recoveringOneRecordWritesOnlyThatOpaqueArtifactToProtectedOutput() throws Exception {
        Path output = temporary.resolve("recovered.json");
        SessionSnapshotTool.recover(validSnapshot(), temporary.resolve("meta"), "target", output);
        String recovered = Files.readString(output);
        assertThat(recovered).contains("target-dex-artifact").doesNotContain("employee-dex-artifact");
        assertThat(Files.getPosixFilePermissions(output).toString()).contains("OWNER_READ", "OWNER_WRITE");
    }

    @Test
    void revokedRecordCannotBeRecovered() {
        assertThatThrownBy(() -> SessionSnapshotTool.recover(validSnapshot(), temporary.resolve("meta"),
                "revoked", temporary.resolve("output"))).hasMessageContaining("expired or revoked");
    }

    @Test
    void rejectsEngineeringAdminAndNoncanonicalGeneralAdminClaims() {
        SessionSnapshotTool.Snapshot alias = validSnapshot();
        alias.records.get(0).identityAlias = "engineering-admin";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(alias)).hasMessageContaining("prohibited identity");

        SessionSnapshotTool.Snapshot group = validSnapshot();
        group.records.get(0).groups = List.of("SCA-Engineering-Admins");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(group)).hasMessageContaining("privileged group");

        for (String variant : List.of("SCA-General-Admins", "General-Admins", "sca-general-admins")) {
            SessionSnapshotTool.Snapshot noncanonical = validSnapshot();
            noncanonical.records.get(1).groups = List.of("SCA-Employees", variant);
            assertThatThrownBy(() -> SessionSnapshotTool.validate(noncanonical))
                    .hasMessageContaining("noncanonical general-admin");
        }
        SessionSnapshotTool.Snapshot caseVariant = validSnapshot();
        caseVariant.records.get(1).groups = List.of("SCA-Employees", "sca-admins");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(caseVariant)).hasMessageContaining("restricted to edge");
    }

    @Test
    void requiresExactlyFourRecordsWithUniqueNonblankIds() {
        SessionSnapshotTool.Snapshot extra = validSnapshot();
        extra.records.add(record("extra", "extra", "revoked", "extra-artifact"));
        assertThatThrownBy(() -> SessionSnapshotTool.validate(extra)).hasMessageContaining("exactly four");
        SessionSnapshotTool.Snapshot duplicate = validSnapshot();
        duplicate.records.get(1).id = "employee";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(duplicate)).hasMessageContaining("unique");
        SessionSnapshotTool.Snapshot blank = validSnapshot();
        blank.records.get(1).id = " ";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(blank)).hasMessageContaining("nonblank");
    }

    @Test
    void requiresNonblankSubjectGroupsCorrelationAndArtifactValues() {
        SessionSnapshotTool.Snapshot subject = validSnapshot(); subject.records.get(0).subject = " ";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(subject)).hasMessageContaining("nonblank");
        SessionSnapshotTool.Snapshot groups = validSnapshot(); groups.records.get(0).groups = List.of();
        assertThatThrownBy(() -> SessionSnapshotTool.validate(groups)).hasMessageContaining("groups");
        SessionSnapshotTool.Snapshot correlation = validSnapshot(); correlation.records.get(0).correlationId = null;
        assertThatThrownBy(() -> SessionSnapshotTool.validate(correlation)).hasMessageContaining("correlation");
        SessionSnapshotTool.Snapshot artifact = validSnapshot(); artifact.records.get(0).artifacts = Map.of("session", " ");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(artifact)).hasMessageContaining("artifacts");
    }

    @Test
    void restrictsStatusesAndRequiresCoherentExpiry() {
        SessionSnapshotTool.Snapshot status = validSnapshot(); status.records.get(0).status = "disabled";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(status)).hasMessageContaining("unsupported");
        SessionSnapshotTool.Snapshot active = validSnapshot(); active.records.get(0).expiresAt = Instant.now().minusSeconds(1);
        assertThatThrownBy(() -> SessionSnapshotTool.validate(active)).hasMessageContaining("incoherent");
        SessionSnapshotTool.Snapshot expired = validSnapshot(); expired.records.get(3).status = "expired";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(expired)).hasMessageContaining("incoherent");
    }

    @Test
    void requiresOneLiveTargetWithExactBoundaryGroups() {
        SessionSnapshotTool.Snapshot expiredTarget = validSnapshot(); expiredTarget.records.get(2).status = "revoked";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(expiredTarget)).hasMessageContaining("active unexpired");
        SessionSnapshotTool.Snapshot missingEmployee = validSnapshot(); missingEmployee.records.get(2).groups = List.of("SCA-Engineering-Users");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(missingEmployee)).hasMessageContaining("target group");
        for (String forbidden : List.of("SCA-Engineering-Admins", "SCA-Admins", "Domain Admins", "Vendor-Middleware", "PLC-Operators")) {
            SessionSnapshotTool.Snapshot snapshot = validSnapshot();
            snapshot.records.get(2).groups = List.of("SCA-Employees", "SCA-Engineering-Users", forbidden);
            assertThatThrownBy(() -> SessionSnapshotTool.validate(snapshot))
                    .isInstanceOf(IllegalArgumentException.class);
        }
    }

    @Test
    void acceptsChopperAsOrdinaryEmployeeByAuthorizationBoundary() {
        SessionSnapshotTool.validate(validSnapshot());
    }

    @Test
    void rejectsPrivilegedOrdinaryEmployeeAndRequiresEdgeAndRetiredRoles() {
        SessionSnapshotTool.Snapshot employee = validSnapshot();
        employee.records.get(0).groups = List.of("SCA-Employees", "SCA-Admins");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(employee)).hasMessageContaining("restricted to edge");
        SessionSnapshotTool.Snapshot target = validSnapshot();
        target.records.get(2).groups = List.of("SCA-Employees", "SCA-Engineering-Users", "SCA-Admins");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(target)).hasMessageContaining("restricted to edge");
        SessionSnapshotTool.Snapshot retiredAdmin = validSnapshot();
        retiredAdmin.records.get(3).groups = List.of("SCA-Employees", "SCA-Admins");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(retiredAdmin)).hasMessageContaining("restricted to edge");
        SessionSnapshotTool.Snapshot edgeMissingAdmin = validSnapshot();
        edgeMissingAdmin.records.get(1).groups = List.of("SCA-Employees");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(edgeMissingAdmin)).hasMessageContaining("edge baseline/admin");
        SessionSnapshotTool.Snapshot edgeMissingEmployee = validSnapshot();
        edgeMissingEmployee.records.get(1).groups = List.of("SCA-Admins");
        assertThatThrownBy(() -> SessionSnapshotTool.validate(edgeMissingEmployee)).hasMessageContaining("edge baseline/admin");
        for (String forbidden : List.of("SCA-Engineering-Users", "Domain-Admins", "Enterprise Admins",
                "Vendor-Middleware", "PLC-Operators")) {
            SessionSnapshotTool.Snapshot edge = validSnapshot();
            edge.records.get(1).groups = List.of("SCA-Employees", "SCA-Admins", forbidden);
            assertThatThrownBy(() -> SessionSnapshotTool.validate(edge))
                    .isInstanceOf(IllegalArgumentException.class);
        }
        SessionSnapshotTool.Snapshot retired = validSnapshot(); retired.records.get(3).status = "active";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(retired)).hasMessageContaining("expired or revoked");
    }

    @Test
    void requiresTargetRequestOperationAndVendorSyncOrDiagnosticCorrelation() {
        SessionSnapshotTool.Snapshot request = validSnapshot(); request.records.get(2).requestId = "2004";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(request)).hasMessageContaining("correlation");
        SessionSnapshotTool.Snapshot operation = validSnapshot(); operation.records.get(2).operation = "DAYWATCH";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(operation)).hasMessageContaining("correlation");
        SessionSnapshotTool.Snapshot correlation = validSnapshot(); correlation.records.get(2).correlationId = "vendor-request-2003";
        assertThatThrownBy(() -> SessionSnapshotTool.validate(correlation)).hasMessageContaining("correlation");
    }

    @Test
    void tamperingFailsGcmAuthentication() throws Exception {
        Path key = temporary.resolve("key");
        Path encrypted = temporary.resolve("snapshot");
        SessionSnapshotTool.keygen(key);
        SessionSnapshotTool.writeSnapshot(validSnapshot(), encrypted, temporary.resolve("meta"), key);
        byte[] bytes = Files.readAllBytes(encrypted);
        bytes[bytes.length - 1] ^= 1;
        Files.write(encrypted, bytes);
        assertThatThrownBy(() -> SessionSnapshotTool.readSnapshot(encrypted, key)).isInstanceOf(Exception.class);
    }

    private SessionSnapshotTool.Snapshot validSnapshot() {
        SessionSnapshotTool.Snapshot snapshot = new SessionSnapshotTool.Snapshot();
        snapshot.findingId = "SCA-VENDOR-009";
        snapshot.records = new ArrayList<>();
        snapshot.records.add(record("employee", "chopper", "active", "employee-dex-artifact"));
        SessionSnapshotTool.Record edge = record("edge", "edge", "active", "edge-dex-artifact");
        edge.groups = List.of("SCA-Employees", "SCA-Admins");
        snapshot.records.add(edge);
        SessionSnapshotTool.Record target = record("target", "swordsman", "active", "target-dex-artifact");
        target.groups = List.of("SCA-Employees", "SCA-Engineering-Users");
        target.requestId = "2003"; target.operation = "NIGHTWATCH"; target.correlationId = "vendor-sync-diagnostic-2003";
        snapshot.records.add(target);
        snapshot.records.add(record("revoked", "former-employee", "revoked", "revoked-dex-artifact"));
        return snapshot;
    }

    private SessionSnapshotTool.Record record(String id, String alias, String status, String artifact) {
        SessionSnapshotTool.Record record = new SessionSnapshotTool.Record();
        record.id = id; record.identityAlias = alias; record.subject = "opaque-subject";
        record.groups = List.of("SCA-Employees"); record.status = status;
        record.expiresAt = Instant.now().plusSeconds(3600); record.correlationId = "vendor-record-" + id;
        record.artifacts = Map.of("session", artifact);
        return record;
    }
}
