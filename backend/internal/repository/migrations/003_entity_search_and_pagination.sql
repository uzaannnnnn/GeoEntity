CREATE VIRTUAL TABLE entity_search USING fts5(
    name,
    description,
    content='entities',
    content_rowid='rowid',
    tokenize='trigram'
);

CREATE TRIGGER entities_search_ai AFTER INSERT ON entities BEGIN
    INSERT INTO entity_search(rowid,name,description)
    VALUES(new.rowid,new.name,new.description);
END;

CREATE TRIGGER entities_search_ad AFTER DELETE ON entities BEGIN
    INSERT INTO entity_search(entity_search,rowid,name,description)
    VALUES('delete',old.rowid,old.name,old.description);
END;

CREATE TRIGGER entities_search_au AFTER UPDATE OF name,description ON entities BEGIN
    INSERT INTO entity_search(entity_search,rowid,name,description)
    VALUES('delete',old.rowid,old.name,old.description);
    INSERT INTO entity_search(rowid,name,description)
    VALUES(new.rowid,new.name,new.description);
END;

INSERT INTO entity_search(entity_search) VALUES('rebuild');

CREATE INDEX idx_entities_type_created_at_id
    ON entities(type,created_at,id);

CREATE INDEX idx_entities_status_created_at_id
    ON entities(status,created_at,id);
