package entities

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GalleryPhoto representa uma foto adicional anexada a uma entidade (qualquer
// kind). Distinta da foto primária (PersonAttrs.PhotoPath / PlaceAttrs.PhotoPath).
// O storage usa o filename PhotoPath (ex.: "<photo_uuid>.jpg") sob PHOTO_DIR.
type GalleryPhoto struct {
	ID        string
	EntityID  string
	PhotoPath string
	Caption   string
	MIME      string
	Ord       int
	CreatedAt time.Time
	CreatedBy string
	UpdatedAt time.Time
	UpdatedBy string
}

// NewGalleryPhoto é o input do AddGalleryPhoto.
type NewGalleryPhoto struct {
	EntityID  string
	PhotoPath string
	Caption   string
	MIME      string
}

// ListGalleryPhotos devolve as fotos vivas (deleted_at IS NULL) de uma
// entidade, ordenadas por (ord ASC, created_at ASC).
func (r *Repo) ListGalleryPhotos(ctx context.Context, entityID string) ([]GalleryPhoto, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entity_id, photo_path, caption, mime, ord,
		       created_at, created_by, updated_at, updated_by
		  FROM app.entity_photos
		 WHERE entity_id = $1 AND deleted_at IS NULL
		 ORDER BY ord ASC, created_at ASC`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GalleryPhoto{}
	for rows.Next() {
		var g GalleryPhoto
		if err := rows.Scan(&g.ID, &g.EntityID, &g.PhotoPath, &g.Caption, &g.MIME,
			&g.Ord, &g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// FindGalleryPhoto devolve uma foto pelo seu ID (ainda que soft-deletada — o
// caller decide se rejeita). EntityID é checado para evitar leitura cruzada.
func (r *Repo) FindGalleryPhoto(ctx context.Context, entityID, photoID string) (*GalleryPhoto, error) {
	var g GalleryPhoto
	var deletedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, entity_id, photo_path, caption, mime, ord,
		       created_at, created_by, updated_at, updated_by, deleted_at
		  FROM app.entity_photos
		 WHERE id = $1 AND entity_id = $2`, photoID, entityID,
	).Scan(&g.ID, &g.EntityID, &g.PhotoPath, &g.Caption, &g.MIME, &g.Ord,
		&g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if deletedAt.Valid {
		return nil, ErrNotFound
	}
	return &g, nil
}

// AddGalleryPhoto insere uma nova foto na galeria. O ord recebido vai como
// está; o caller é responsável por gerá-lo (default 0 = anexar ao fim em
// termos práticos, já que rompemos empate por created_at). Bumpa version
// da entidade base. Devolve o registro persistido.
func (r *Repo) AddGalleryPhoto(ctx context.Context, in NewGalleryPhoto, ord int, createdBy string) (*GalleryPhoto, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	// Verifica que a entidade existe e está viva.
	var exists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM app.entities WHERE id = $1 AND deleted_at IS NULL)`,
		in.EntityID,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	var g GalleryPhoto
	err = tx.QueryRowContext(ctx, `
		INSERT INTO app.entity_photos
		  (entity_id, photo_path, caption, mime, ord,
		   created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, entity_id, photo_path, caption, mime, ord,
		          created_at, created_by, updated_at, updated_by`,
		in.EntityID, in.PhotoPath, in.Caption, in.MIME, ord, createdBy,
	).Scan(&g.ID, &g.EntityID, &g.PhotoPath, &g.Caption, &g.MIME, &g.Ord,
		&g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app.entities
		   SET version = version + 1, updated_at = now(), updated_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		createdBy, in.EntityID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return &g, nil
}

// UpdateGalleryCaption altera a caption (e opcionalmente ord) de uma foto da
// galeria. Bumpa version da entidade. Devolve o registro atualizado. Não
// modifica o arquivo em disco.
func (r *Repo) UpdateGalleryCaption(ctx context.Context, entityID, photoID, caption string, ord *int, updatedBy string) (*GalleryPhoto, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	var g GalleryPhoto
	var ordParam sql.NullInt64
	if ord != nil {
		ordParam = sql.NullInt64{Int64: int64(*ord), Valid: true}
	}
	err = tx.QueryRowContext(ctx, `
		UPDATE app.entity_photos
		   SET caption    = $1,
		       ord        = COALESCE($2, ord),
		       updated_at = now(),
		       updated_by = $3
		 WHERE id = $4 AND entity_id = $5 AND deleted_at IS NULL
		 RETURNING id, entity_id, photo_path, caption, mime, ord,
		           created_at, created_by, updated_at, updated_by`,
		caption, ordParam, updatedBy, photoID, entityID,
	).Scan(&g.ID, &g.EntityID, &g.PhotoPath, &g.Caption, &g.MIME, &g.Ord,
		&g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app.entities
		   SET version = version + 1, updated_at = now(), updated_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		updatedBy, entityID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rollback = false
	return &g, nil
}

// SoftDeleteGalleryPhoto marca a foto como deletada. Retorna o PhotoPath
// (filename) para o caller remover o arquivo do disco, e bumpa a version
// da entidade. Idempotente: se já estava deletada, retorna ErrNotFound.
func (r *Repo) SoftDeleteGalleryPhoto(ctx context.Context, entityID, photoID, deletedBy string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	var photoPath string
	err = tx.QueryRowContext(ctx, `
		UPDATE app.entity_photos
		   SET deleted_at = now(),
		       deleted_by = $1,
		       updated_at = now(),
		       updated_by = $1
		 WHERE id = $2 AND entity_id = $3 AND deleted_at IS NULL
		 RETURNING photo_path`,
		deletedBy, photoID, entityID,
	).Scan(&photoPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app.entities
		   SET version = version + 1, updated_at = now(), updated_by = $1
		 WHERE id = $2 AND deleted_at IS NULL`,
		deletedBy, entityID,
	); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	rollback = false
	return photoPath, nil
}
