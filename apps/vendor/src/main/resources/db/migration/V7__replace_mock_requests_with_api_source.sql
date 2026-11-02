alter table maintenance_requests
    add column remote_acknowledged boolean not null default false;

-- Restore only inventory deducted by runtime shipments for the original fixture orders.
update inventory i
set quantity_on_hand = i.quantity_on_hand + restored.quantity,
    updated_at = now()
from (
    select ia.inventory_id, sum(-ia.quantity_delta)::integer as quantity
    from inventory_adjustments ia
    where ia.adjustment_type = 'SHIPMENT'
      and ia.reason in (
          select 'Shipment for ' || fo.order_number
          from fulfillment_orders fo
          join maintenance_requests mr on mr.id = fo.maintenance_request_id
          where mr.external_request_id in (
              'ENG-REQ-2401', 'ENG-REQ-2402', 'ENG-REQ-2403', 'ENG-REQ-2404',
              'ENG-REQ-2405', 'ENG-REQ-2406', 'ENG-REQ-2407', 'ENG-REQ-2408'
          )
      )
    group by ia.inventory_id
) restored
where i.id = restored.inventory_id;

delete from inventory_adjustments
where adjustment_type = 'SHIPMENT'
  and reason in (
      select 'Shipment for ' || fo.order_number
      from fulfillment_orders fo
      join maintenance_requests mr on mr.id = fo.maintenance_request_id
      where mr.external_request_id in (
          'ENG-REQ-2401', 'ENG-REQ-2402', 'ENG-REQ-2403', 'ENG-REQ-2404',
          'ENG-REQ-2405', 'ENG-REQ-2406', 'ENG-REQ-2407', 'ENG-REQ-2408'
      )
  );

delete from shipment_jobs
where maintenance_request_id in (select id from maintenance_requests where external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408'))
   or fulfillment_order_id in (
       select fo.id from fulfillment_orders fo
       join maintenance_requests mr on mr.id = fo.maintenance_request_id
       where mr.external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408')
   );

delete from sync_events
where maintenance_request_id in (select id from maintenance_requests where external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408'));

delete from fulfillment_events
where maintenance_request_id in (select id from maintenance_requests where external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408'));

delete from shipments
where fulfillment_order_id in (
    select fo.id from fulfillment_orders fo
    join maintenance_requests mr on mr.id = fo.maintenance_request_id
    where mr.external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408')
);

delete from fulfillment_order_items
where fulfillment_order_id in (
    select fo.id from fulfillment_orders fo
    join maintenance_requests mr on mr.id = fo.maintenance_request_id
    where mr.external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408')
);

delete from fulfillment_orders
where maintenance_request_id in (select id from maintenance_requests where external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408'));

delete from maintenance_requests where external_request_id in ('ENG-REQ-2401','ENG-REQ-2402','ENG-REQ-2403','ENG-REQ-2404','ENG-REQ-2405','ENG-REQ-2406','ENG-REQ-2407','ENG-REQ-2408');
