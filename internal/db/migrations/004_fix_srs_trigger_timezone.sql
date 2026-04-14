-- +goose Up
DROP TRIGGER IF EXISTS trg_init_problem_srs;

-- +goose StatementBegin
CREATE TRIGGER trg_init_problem_srs
AFTER INSERT ON problems
BEGIN
    INSERT INTO problem_srs (problem_id, next_review_date)
    VALUES (NEW.id, date('now', 'localtime'));
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_init_problem_srs;

-- +goose StatementBegin
CREATE TRIGGER trg_init_problem_srs
AFTER INSERT ON problems
BEGIN
    INSERT INTO problem_srs (problem_id) VALUES (NEW.id);
END;
-- +goose StatementEnd
