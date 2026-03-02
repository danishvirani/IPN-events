ALTER TABLE events ADD COLUMN city TEXT;
ALTER TABLE events ADD COLUMN scope TEXT NOT NULL DEFAULT 'regional';
ALTER TABLE events ADD COLUMN scope_jamatkhana TEXT;
ALTER TABLE events ADD COLUMN venue_type TEXT NOT NULL DEFAULT 'external';
ALTER TABLE events ADD COLUMN venue_jamatkhana TEXT;
ALTER TABLE events ADD COLUMN venue_address TEXT;

ALTER TABLE event_support_requests ADD COLUMN venue_type TEXT;
ALTER TABLE event_support_requests ADD COLUMN venue_detail TEXT;
