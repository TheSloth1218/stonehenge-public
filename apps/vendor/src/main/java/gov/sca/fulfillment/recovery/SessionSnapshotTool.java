package gov.sca.fulfillment.recovery;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;

import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.io.InputStream;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.Base64;
import java.util.List;
import java.util.Locale;
import java.util.Set;
import java.util.HashSet;

/** Root-only, instructor-facing SCA-VENDOR-009 snapshot utility. */
public final class SessionSnapshotTool {
    static final byte[] MAGIC = "SVS00901".getBytes(StandardCharsets.US_ASCII);
    static final Set<String> PROHIBITED_ALIASES = Set.of("administrator", "admin", "engineering-admin", "engadmin");
    static final Set<String> PROHIBITED_GROUP_FRAGMENTS = Set.of("engineering-admin", "domain admin", "enterprise admin");
    static final Set<String> ALLOWED_STATUSES = Set.of("active", "expired", "revoked");
    private static final ObjectMapper JSON = new ObjectMapper().findAndRegisterModules()
            .enable(SerializationFeature.INDENT_OUTPUT);

    private SessionSnapshotTool() { }

    public static void main(String[] args) throws Exception {
        requireRoot();
        if (args.length < 4) usage();
        String command = args[0];
        Path snapshot = Path.of(args[1]);
        Path metadata = Path.of(args[2]);
        Path key = Path.of(args[3]);
        switch (command) {
            case "keygen" -> keygen(key);
            case "import" -> writeSnapshot(JSON.readValue(System.in, Snapshot.class), snapshot, metadata, key);
            case "list" -> list(readSnapshot(snapshot, key));
            case "recover" -> {
                if (args.length != 7 || !"--output".equals(args[5])) usage();
                recover(readSnapshot(snapshot, key), metadata, args[4], Path.of(args[6]));
            }
            case "validate" -> { validate(readSnapshot(snapshot, key)); System.out.println("snapshot valid; secrets not displayed"); }
            default -> usage();
        }
    }

    static void keygen(Path key) throws Exception {
        if (Files.exists(key)) throw new IllegalStateException("refusing to overwrite recovery key");
        KeyGenerator generator = KeyGenerator.getInstance("AES");
        generator.init(256);
        Files.createDirectories(key.getParent());
        Files.write(key, generator.generateKey().getEncoded(), StandardOpenOption.CREATE_NEW);
        setOwnerOnly(key);
        System.out.println("recovery key created; contents not displayed");
    }

    static void writeSnapshot(Snapshot data, Path snapshot, Path metadata, Path key) throws Exception {
        validate(data);
        SecretKey secretKey = readKey(key);
        byte[] nonce = new byte[12];
        new SecureRandom().nextBytes(nonce);
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.ENCRYPT_MODE, secretKey, new GCMParameterSpec(128, nonce));
        cipher.updateAAD(MAGIC);
        byte[] encrypted = cipher.doFinal(JSON.writeValueAsBytes(data));
        ByteBuffer result = ByteBuffer.allocate(MAGIC.length + nonce.length + encrypted.length)
                .put(MAGIC).put(nonce).put(encrypted);
        Files.createDirectories(snapshot.getParent());
        Files.write(snapshot, result.array());
        setOwnerOnly(snapshot);
        Metadata safe = metadata(data);
        Files.write(metadata, JSON.writeValueAsBytes(safe));
        setOwnerOnly(metadata);
        System.out.printf("snapshot imported: %d records; finding=%s%n", data.records.size(), data.findingId);
    }

    static Snapshot readSnapshot(Path snapshot, Path key) throws Exception {
        byte[] value = Files.readAllBytes(snapshot);
        if (value.length < MAGIC.length + 12 + 16) throw new IllegalArgumentException("invalid snapshot");
        ByteBuffer bytes = ByteBuffer.wrap(value);
        byte[] magic = new byte[MAGIC.length]; bytes.get(magic);
        if (!MessageDigest.isEqual(MAGIC, magic)) throw new IllegalArgumentException("invalid snapshot format");
        byte[] nonce = new byte[12]; bytes.get(nonce);
        byte[] encrypted = new byte[bytes.remaining()]; bytes.get(encrypted);
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.DECRYPT_MODE, readKey(key), new GCMParameterSpec(128, nonce));
        cipher.updateAAD(MAGIC);
        Snapshot result = JSON.readValue(cipher.doFinal(encrypted), Snapshot.class);
        validate(result);
        return result;
    }

    static void recover(Snapshot snapshot, Path ignored, String requested, Path output) throws Exception {
        Record record = snapshot.records.stream().filter(r -> r.id.equals(requested)).findFirst()
                .orElseThrow(() -> new IllegalArgumentException("record not found"));
        if (!"active".equals(record.status) || !record.expiresAt.isAfter(Instant.now()))
            throw new IllegalStateException("record is expired or revoked");
        Files.write(output, JSON.writeValueAsBytes(record.artifacts), StandardOpenOption.CREATE_NEW);
        setOwnerOnly(output);
        System.out.printf("record=%s expiry=%s status=%s finding=%s correlation=%s tokenHash=%s%n",
                record.id, record.expiresAt, record.status, snapshot.findingId, record.correlationId, hashPrefix(record.artifacts));
    }

    static void validate(Snapshot snapshot) {
        if (snapshot == null || !"SCA-VENDOR-009".equals(snapshot.findingId)
                || snapshot.records == null || snapshot.records.size() != 4)
            throw new IllegalArgumentException("finding ID and exactly four bounded records are required");
        Instant now = Instant.now();
        Set<String> ids = new HashSet<>();
        for (Record record : snapshot.records) {
            if (record == null || blank(record.id) || blank(record.identityAlias) || blank(record.subject)
                    || record.expiresAt == null || blank(record.status) || blank(record.correlationId)
                    || record.groups == null || record.groups.isEmpty() || record.groups.stream().anyMatch(SessionSnapshotTool::blank)
                    || record.artifacts == null || record.artifacts.isEmpty()
                    || record.artifacts.entrySet().stream().anyMatch(e -> blank(e.getKey()) || blank(e.getValue())))
                throw new IllegalArgumentException("nonblank record, subject, groups, correlation, and opaque artifacts are required");
            if (!ids.add(record.id)) throw new IllegalArgumentException("record IDs must be unique");
            if (!record.id.matches("[A-Za-z0-9._-]+") || !record.correlationId.matches("[A-Za-z0-9._:-]+"))
                throw new IllegalArgumentException("record ID and correlation must be sanitized identifiers");
            if (!ALLOWED_STATUSES.contains(record.status)) throw new IllegalArgumentException("unsupported record status");
            if (("active".equals(record.status) && !record.expiresAt.isAfter(now))
                    || ("expired".equals(record.status) && record.expiresAt.isAfter(now)))
                throw new IllegalArgumentException("record status and expiry are incoherent");
            String alias = record.identityAlias.toLowerCase(Locale.ROOT);
            if (PROHIBITED_ALIASES.contains(alias) || alias.contains("engineering") || alias.contains("admin"))
                throw new IllegalArgumentException("prohibited identity alias");
            for (String group : record.groups == null ? List.<String>of() : record.groups) {
                String normalized = group.toLowerCase(Locale.ROOT);
                if (PROHIBITED_GROUP_FRAGMENTS.stream().anyMatch(normalized::contains))
                    throw new IllegalArgumentException("privileged group claim rejected");
                if (normalized.contains("general-admin"))
                    throw new IllegalArgumentException("noncanonical general-admin group claim rejected");
                if (normalized.equals("sca-admins") && (!"SCA-Admins".equals(group) || !"edge".equals(alias)))
                    throw new IllegalArgumentException("SCA-Admins is restricted to edge");
            }
        }
        List<Record> targets = snapshot.records.stream().filter(r -> "swordsman".equals(r.identityAlias)).toList();
        if (targets.size() != 1 || !"active".equals(targets.get(0).status) || !targets.get(0).expiresAt.isAfter(now))
            throw new IllegalArgumentException("exactly one active unexpired swordsman target is required");
        Record target = targets.get(0);
        Set<String> targetGroups = Set.copyOf(target.groups);
        if (!targetGroups.containsAll(Set.of("SCA-Employees", "SCA-Engineering-Users"))
                || target.groups.stream().map(g -> g.toLowerCase(Locale.ROOT)).anyMatch(g ->
                g.contains("engineering-admin") || g.equals("sca-admins") || g.equals("domain admins")
                        || g.contains("middleware") || g.contains("plc")))
            throw new IllegalArgumentException("target group boundary is invalid");
        String correlation = target.correlationId.toLowerCase(Locale.ROOT);
        if (!"2003".equals(target.requestId) || !"NIGHTWATCH".equals(target.operation)
                || !correlation.contains("vendor") || !(correlation.contains("sync") || correlation.contains("diagnostic")))
            throw new IllegalArgumentException("target correlation is incomplete");
        if (snapshot.records.stream().noneMatch(r -> r != target && ordinaryEmployeeBoundary(r, now)))
            throw new IllegalArgumentException("a distinct ordinary employee non-target is required");
        if (snapshot.records.stream().noneMatch(r -> r != target && "edge".equals(normalizeGroup(r.identityAlias))
                && Set.copyOf(r.groups).containsAll(Set.of("SCA-Employees", "SCA-Admins"))
                && r.groups.stream().map(SessionSnapshotTool::normalizeGroup).noneMatch(group ->
                group.contains("engineering") || group.contains("domain admin") || group.contains("domain-admin")
                        || group.contains("enterprise admin") || group.contains("enterprise-admin")
                        || group.contains("middleware") || group.contains("plc"))))
            throw new IllegalArgumentException("edge baseline/admin group boundary is invalid");
        if (snapshot.records.stream().noneMatch(r -> "expired".equals(r.status) || "revoked".equals(r.status)))
            throw new IllegalArgumentException("an expired or revoked record is required");
    }

    private static boolean blank(String value) { return value == null || value.isBlank(); }
    private static String normalizeGroup(String value) { return value.toLowerCase(Locale.ROOT); }
    private static boolean ordinaryEmployeeBoundary(Record record, Instant now) {
        if (!"active".equals(record.status) || !record.expiresAt.isAfter(now)
                || !record.groups.contains("SCA-Employees")) return false;
        return record.groups.stream().map(SessionSnapshotTool::normalizeGroup).noneMatch(group ->
                group.equals("sca-admins") || group.contains("general-admin") || group.contains("engineering")
                        || group.contains("domain admin") || group.contains("domain-admin")
                        || group.contains("enterprise admin") || group.contains("enterprise-admin")
                        || group.contains("middleware") || group.contains("plc"));
    }

    private static Metadata metadata(Snapshot snapshot) {
        return new Metadata(snapshot.findingId, snapshot.records.stream()
                .map(r -> new MetadataRecord(r.id, r.identityAlias, r.expiresAt, r.status, r.correlationId,
                        hashPrefix(r.artifacts))).toList());
    }

    private static void list(Snapshot snapshot) {
        for (Record r : snapshot.records)
            System.out.printf("record=%s expiry=%s status=%s finding=%s correlation=%s tokenHash=%s%n",
                    r.id, r.expiresAt, r.status, snapshot.findingId, r.correlationId, hashPrefix(r.artifacts));
    }

    private static SecretKey readKey(Path key) throws Exception {
        byte[] bytes = Files.readAllBytes(key);
        if (bytes.length != 32) throw new IllegalArgumentException("recovery key must be 256-bit AES");
        return new SecretKeySpec(bytes, "AES");
    }

    private static String hashPrefix(Object value) {
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256").digest(JSON.writeValueAsBytes(value));
            return Base64.getUrlEncoder().withoutPadding().encodeToString(digest).substring(0, 12);
        } catch (Exception e) { throw new IllegalStateException(e); }
    }

    private static void setOwnerOnly(Path path) throws Exception {
        try { Files.setPosixFilePermissions(path, java.nio.file.attribute.PosixFilePermissions.fromString("rw-------")); }
        catch (UnsupportedOperationException e) { throw new IllegalStateException("POSIX permissions required", e); }
    }

    private static void requireRoot() throws Exception {
        try (InputStream input = Files.newInputStream(Path.of("/proc/self/status"))) {
            String status = new String(input.readAllBytes(), StandardCharsets.US_ASCII);
            if (!status.lines().anyMatch(line -> line.matches("Uid:\\s+0\\s+0\\s+0\\s+0")))
                throw new SecurityException("session recovery must run as root");
        }
    }

    private static void usage() { throw new IllegalArgumentException("usage: keygen|import|list|validate SNAPSHOT META KEY; recover SNAPSHOT META KEY RECORD --output PATH"); }

    @JsonIgnoreProperties(ignoreUnknown = false)
    public static final class Snapshot { public String findingId; public List<Record> records; }
    @JsonIgnoreProperties(ignoreUnknown = false)
    public static final class Record {
        public String id; public String identityAlias; public String subject; public List<String> groups;
        public Instant expiresAt; public String status; public String requestId; public String operation;
        public String correlationId; public java.util.Map<String, String> artifacts;
    }
    record Metadata(String findingId, List<MetadataRecord> records) { }
    record MetadataRecord(String id, String identityAlias, Instant expiresAt, String status, String correlationId, String tokenHashPrefix) { }
}
