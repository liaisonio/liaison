-- Destructive rollback: removes WebIDE registration metadata, not device files.
DROP TABLE IF EXISTS webide_accesses;
DROP TABLE IF EXISTS webide_applications;
