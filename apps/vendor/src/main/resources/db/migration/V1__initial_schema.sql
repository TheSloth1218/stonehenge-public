create table users (
    id bigserial primary key,
    username varchar(80) not null unique,
    display_name varchar(160) not null,
    email varchar(200) not null,
    role varchar(60) not null,
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    last_login_at timestamptz
);

create table maintenance_requests (
    id bigserial primary key,
    external_request_id varchar(120) not null unique,
    title varchar(240) not null,
    description text,
    source_asset varchar(160),
    source_panel varchar(160),
    priority varchar(40) not null,
    remote_status varchar(80) not null,
    local_status varchar(80) not null,
    requested_at timestamptz not null,
    imported_at timestamptz not null default now(),
    last_synced_at timestamptz,
    assigned_user_id bigint references users(id),
    raw_snapshot_json jsonb not null default '{}'::jsonb,
    version bigint not null default 0
);

create table parts (
    id bigserial primary key,
    sku varchar(80) not null unique,
    name varchar(180) not null,
    description text,
    category varchar(100) not null,
    manufacturer varchar(120),
    compatible_asset_class varchar(120),
    image_path varchar(240),
    active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    version bigint not null default 0
);

create table inventory (
    id bigserial primary key,
    part_id bigint not null unique references parts(id),
    warehouse_location varchar(120) not null,
    bin_location varchar(80) not null,
    quantity_on_hand integer not null check (quantity_on_hand >= 0),
    quantity_reserved integer not null default 0 check (quantity_reserved >= 0),
    reorder_level integer not null default 0 check (reorder_level >= 0),
    updated_at timestamptz not null default now(),
    version bigint not null default 0,
    check (quantity_reserved <= quantity_on_hand)
);

create table fulfillment_orders (
    id bigserial primary key,
    maintenance_request_id bigint not null unique references maintenance_requests(id),
    order_number varchar(80) not null unique,
    status varchar(80) not null,
    assigned_user_id bigint references users(id),
    fulfillment_notes text,
    packing_notes text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    reviewed_at timestamptz,
    shipped_at timestamptz,
    completed_at timestamptz,
    version bigint not null default 0
);

create table fulfillment_order_items (
    id bigserial primary key,
    fulfillment_order_id bigint not null references fulfillment_orders(id) on delete cascade,
    part_id bigint not null references parts(id),
    quantity integer not null check (quantity > 0),
    added_by_user_id bigint references users(id),
    added_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    version bigint not null default 0,
    unique (fulfillment_order_id, part_id)
);

create table shipments (
    id bigserial primary key,
    fulfillment_order_id bigint not null unique references fulfillment_orders(id),
    shipment_number varchar(80) not null unique,
    carrier varchar(120) not null,
    shipping_method varchar(120) not null,
    tracking_number varchar(120) not null unique,
    destination_label varchar(240) not null,
    packed_by_user_id bigint references users(id),
    shipped_by_user_id bigint references users(id),
    shipped_at timestamptz not null,
    notes text,
    created_at timestamptz not null default now()
);

create table inventory_adjustments (
    id bigserial primary key,
    inventory_id bigint not null references inventory(id),
    adjustment_type varchar(80) not null,
    quantity_delta integer not null,
    previous_quantity integer not null,
    resulting_quantity integer not null check (resulting_quantity >= 0),
    reason varchar(400) not null,
    created_by_user_id bigint references users(id),
    created_at timestamptz not null default now()
);

create table fulfillment_events (
    id bigserial primary key,
    maintenance_request_id bigint references maintenance_requests(id),
    fulfillment_order_id bigint references fulfillment_orders(id),
    event_type varchar(100) not null,
    actor_user_id bigint references users(id),
    summary varchar(400) not null,
    details_json jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);

create table sync_events (
    id bigserial primary key,
    maintenance_request_id bigint references maintenance_requests(id),
    fulfillment_order_id bigint references fulfillment_orders(id),
    operation varchar(120) not null,
    direction varchar(40) not null,
    success boolean not null,
    http_status integer,
    error_code varchar(100),
    sanitized_message varchar(1000),
    correlation_id varchar(120) not null,
    attempted_at timestamptz not null,
    completed_at timestamptz
);

create index idx_requests_status on maintenance_requests(local_status);
create index idx_parts_category on parts(category);
create index idx_orders_status on fulfillment_orders(status);
create index idx_events_created_at on fulfillment_events(created_at desc);
create index idx_sync_attempted_at on sync_events(attempted_at desc);
