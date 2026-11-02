insert into users (username, display_name, email, role, enabled)
values
('operator', 'Avery Stone', 'avery.stone@sca.internal', 'FULFILLMENT_OPERATOR', true),
('supervisor', 'Morgan Vale', 'morgan.vale@sca.internal', 'WAREHOUSE_SUPERVISOR', true),
('admin', 'Jordan Keene', 'jordan.keene@sca.internal', 'LOGISTICS_ADMIN', true);

insert into parts (sku, name, description, category, manufacturer, compatible_asset_class, image_path, active)
values
('SCA-AZ-4100', 'Azimuth Servo Assembly', 'Servo assembly used for azimuth positioning maintenance kits.', 'Drive Systems', 'North Ridge Motion', 'Gun Mount', '/css/part-placeholder.svg', true),
('SCA-EL-2200', 'Elevation Drive Motor', 'Sealed drive motor for elevation axis service.', 'Drive Systems', 'North Ridge Motion', 'Gun Mount', '/css/part-placeholder.svg', true),
('SCA-SR-1204', 'Dual-Channel Safety Relay', 'DIN rail safety relay for guarded maintenance circuits.', 'Controls', 'Hale Industrial', 'Control Cabinet', '/css/part-placeholder.svg', true),
('SCA-IO-8008', 'Control I/O Module', 'Eight-channel isolated I/O replacement module.', 'Controls', 'Hale Industrial', 'Control Cabinet', '/css/part-placeholder.svg', true),
('SCA-HY-3310', 'Hydraulic Seal Pack', 'Seal kit for hydraulic service manifold maintenance.', 'Hydraulics', 'Aster Fluidics', 'Hydraulic Skid', '/css/part-placeholder.svg', true),
('SCA-CH-0900', 'Coolant Hose Kit', 'Pre-cut coolant hose set with clamps.', 'Fluid Handling', 'Aster Fluidics', 'Support System', '/css/part-placeholder.svg', true),
('SCA-EN-7712', 'Encoder Replacement Kit', 'Encoder, coupler, and fastener set for position feedback service.', 'Sensors', 'Orbital Instrument', 'Gun Mount', '/css/part-placeholder.svg', true),
('SCA-PD-6200', 'Power Distribution Module', 'Fused panel distribution module for control cabinet service.', 'Electrical', 'Kepler Power', 'Control Cabinet', '/css/part-placeholder.svg', true),
('SCA-MF-4400', 'Maintenance Fastener Kit', 'Assorted approved fasteners for field maintenance tasks.', 'Hardware', 'Summit Fabrication', 'General', '/css/part-placeholder.svg', true),
('SCA-NW-1500', 'Industrial Network Transceiver', 'Ruggedized fiber-to-copper transceiver for cabinet networks.', 'Networking', 'Kepler Power', 'Control Cabinet', '/css/part-placeholder.svg', true),
('SCA-FL-2280', 'Inline Filter Element', 'Replacement filter element for coolant return loop.', 'Fluid Handling', 'Aster Fluidics', 'Support System', '/css/part-placeholder.svg', true),
('SCA-LK-5100', 'Legacy Limit Switch', 'Retired limit switch kept for historical shipped orders only.', 'Sensors', 'Orbital Instrument', 'Gun Mount', '/css/part-placeholder.svg', false);

insert into inventory (part_id, warehouse_location, bin_location, quantity_on_hand, quantity_reserved, reorder_level)
select id, 'SCA Logistics Cage A', 'A-' || lpad(id::text, 2, '0'),
       case sku
           when 'SCA-AZ-4100' then 4
           when 'SCA-EL-2200' then 3
           when 'SCA-SR-1204' then 12
           when 'SCA-IO-8008' then 8
           when 'SCA-HY-3310' then 20
           when 'SCA-CH-0900' then 2
           when 'SCA-EN-7712' then 5
           when 'SCA-PD-6200' then 6
           when 'SCA-MF-4400' then 30
           when 'SCA-NW-1500' then 7
           when 'SCA-FL-2280' then 1
           else 0
       end,
       0,
       case when sku in ('SCA-CH-0900', 'SCA-FL-2280') then 3 else 2 end
from parts;

insert into maintenance_requests (external_request_id, title, description, source_asset, source_panel, priority, remote_status, local_status, requested_at, imported_at, assigned_user_id, raw_snapshot_json)
values
('ENG-REQ-2401', 'Gun 1 elevation response maintenance', 'Engineering requested drive service parts after a slow elevation response check.', 'Gun 1', 'Elevation Drive Panel', 'HIGH', 'APPROVED', 'NEW', now() - interval '12 hours', now() - interval '11 hours', null, '{"sanitized":true}'::jsonb),
('ENG-REQ-2402', 'Gun 3 cabinet safety relay replacement', 'Approved maintenance request for relay replacement during scheduled downtime.', 'Gun 3', 'Control Cabinet C', 'NORMAL', 'APPROVED', 'IN_PROGRESS', now() - interval '2 days', now() - interval '2 days', 1, '{"sanitized":true}'::jsonb),
('ENG-REQ-2403', 'Gun 5 hydraulic seal service', 'Hydraulic manifold seepage inspection requires seal pack.', 'Gun 5', 'Hydraulic Service Skid', 'HIGH', 'APPROVED', 'READY_TO_SHIP', now() - interval '3 days', now() - interval '3 days', 1, '{"sanitized":true}'::jsonb),
('ENG-REQ-2404', 'Gun 2 coolant hose replacement', 'Coolant hose set requested for preventive maintenance.', 'Gun 2', 'Cooling Support Panel', 'LOW', 'APPROVED', 'FULFILLED', now() - interval '5 days', now() - interval '5 days', 2, '{"sanitized":true}'::jsonb),
('ENG-REQ-2405', 'Gun 7 network link service', 'Industrial network transceiver requested for cabinet link maintenance.', 'Gun 7', 'Control Cabinet G', 'NORMAL', 'APPROVED', 'SYNC_FAILED', now() - interval '1 day', now() - interval '23 hours', 1, '{"sanitized":true}'::jsonb),
('ENG-REQ-2406', 'Gun 4 fastener replenishment', 'Maintenance fastener kit requested for local support shelf.', 'Gun 4', 'Maintenance Bay', 'LOW', 'APPROVED', 'NEW', now() - interval '8 hours', now() - interval '7 hours', null, '{"sanitized":true}'::jsonb),
('ENG-REQ-2407', 'Gun 8 I/O module service kit', 'Control I/O replacement requested after engineering diagnostics.', 'Gun 8', 'Control Cabinet H', 'URGENT', 'APPROVED', 'NEW', now() - interval '3 hours', now() - interval '2 hours', null, '{"sanitized":true}'::jsonb),
('ENG-REQ-2408', 'Gun 6 retired switch request', 'Cancelled engineering request for a retired switch.', 'Gun 6', 'Limit Switch Junction', 'NORMAL', 'CANCELLED', 'CANCELLED', now() - interval '6 days', now() - interval '6 days', 2, '{"sanitized":true}'::jsonb);

insert into fulfillment_orders (maintenance_request_id, order_number, status, assigned_user_id, created_at, updated_at, reviewed_at, shipped_at, completed_at)
select id, 'FO-' || replace(external_request_id, '-', ''), 
       case external_request_id
           when 'ENG-REQ-2402' then 'PICKING'
           when 'ENG-REQ-2403' then 'REVIEW'
           when 'ENG-REQ-2404' then 'COMPLETION_SENT'
           when 'ENG-REQ-2405' then 'COMPLETION_FAILED'
       end,
       assigned_user_id, imported_at, imported_at + interval '1 hour',
       case when external_request_id in ('ENG-REQ-2403', 'ENG-REQ-2404', 'ENG-REQ-2405') then imported_at + interval '2 hours' end,
       case when external_request_id in ('ENG-REQ-2404', 'ENG-REQ-2405') then imported_at + interval '3 hours' end,
       case when external_request_id = 'ENG-REQ-2404' then imported_at + interval '4 hours' end
from maintenance_requests
where external_request_id in ('ENG-REQ-2402', 'ENG-REQ-2403', 'ENG-REQ-2404', 'ENG-REQ-2405');

insert into fulfillment_order_items (fulfillment_order_id, part_id, quantity, added_by_user_id)
select fo.id, p.id, 1, 1 from fulfillment_orders fo join parts p on p.sku = 'SCA-SR-1204' where fo.order_number = 'FO-ENGREQ2402';
insert into fulfillment_order_items (fulfillment_order_id, part_id, quantity, added_by_user_id)
select fo.id, p.id, 2, 1 from fulfillment_orders fo join parts p on p.sku = 'SCA-HY-3310' where fo.order_number = 'FO-ENGREQ2403';
insert into fulfillment_order_items (fulfillment_order_id, part_id, quantity, added_by_user_id)
select fo.id, p.id, 1, 2 from fulfillment_orders fo join parts p on p.sku = 'SCA-CH-0900' where fo.order_number = 'FO-ENGREQ2404';
insert into fulfillment_order_items (fulfillment_order_id, part_id, quantity, added_by_user_id)
select fo.id, p.id, 1, 1 from fulfillment_orders fo join parts p on p.sku = 'SCA-NW-1500' where fo.order_number = 'FO-ENGREQ2405';

insert into shipments (fulfillment_order_id, shipment_number, carrier, shipping_method, tracking_number, destination_label, packed_by_user_id, shipped_by_user_id, shipped_at, notes)
select id, 'SHP-202607090001', 'SCA Internal Logistics', 'Ground Courier', 'SCA7E2A91C40B12', 'Stonehenge Engineering Maintenance Bay', 2, 2, now() - interval '2 days', 'Delivered to maintenance intake.'
from fulfillment_orders where order_number = 'FO-ENGREQ2404';
insert into shipments (fulfillment_order_id, shipment_number, carrier, shipping_method, tracking_number, destination_label, packed_by_user_id, shipped_by_user_id, shipped_at, notes)
select id, 'SHP-202607100001', 'SCA Internal Logistics', 'Priority Courier', 'SCA1D9F5E88A300', 'Stonehenge Engineering Maintenance Bay', 1, 1, now() - interval '20 hours', 'Completion sync pending retry.'
from fulfillment_orders where order_number = 'FO-ENGREQ2405';

insert into fulfillment_events (maintenance_request_id, fulfillment_order_id, event_type, actor_user_id, summary, details_json, created_at)
select mr.id, fo.id, 'FULFILLMENT_STARTED', fo.assigned_user_id, 'Fulfillment order created from imported request.', '{}'::jsonb, fo.created_at
from fulfillment_orders fo join maintenance_requests mr on mr.id = fo.maintenance_request_id;
insert into fulfillment_events (maintenance_request_id, fulfillment_order_id, event_type, actor_user_id, summary, details_json, created_at)
select mr.id, fo.id, 'ORDER_SHIPPED', fo.assigned_user_id, 'Order shipped locally.', '{}'::jsonb, fo.shipped_at
from fulfillment_orders fo join maintenance_requests mr on mr.id = fo.maintenance_request_id where fo.shipped_at is not null;

insert into sync_events (maintenance_request_id, fulfillment_order_id, operation, direction, success, http_status, error_code, sanitized_message, correlation_id, attempted_at, completed_at)
select mr.id, fo.id, 'vendor.part-request.complete', 'OUTBOUND', true, 200, null, 'Completion update accepted by Stonehenge API.', 'seed-sync-success-2404', fo.completed_at, fo.completed_at
from fulfillment_orders fo join maintenance_requests mr on mr.id = fo.maintenance_request_id where fo.order_number = 'FO-ENGREQ2404';
insert into sync_events (maintenance_request_id, fulfillment_order_id, operation, direction, success, http_status, error_code, sanitized_message, correlation_id, attempted_at, completed_at)
select mr.id, fo.id, 'vendor.part-request.complete', 'OUTBOUND', false, null, 'COMPLETION_SYNC_FAILED', 'Internal API completion update timed out. Safe to retry.', 'seed-sync-failed-2405', fo.shipped_at + interval '1 minute', fo.shipped_at + interval '1 minute'
from fulfillment_orders fo join maintenance_requests mr on mr.id = fo.maintenance_request_id where fo.order_number = 'FO-ENGREQ2405';
