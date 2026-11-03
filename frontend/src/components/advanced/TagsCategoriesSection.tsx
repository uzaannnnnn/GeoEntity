import { useState, useEffect, useCallback, useId, useRef } from 'react';
import { Entity } from '../../types/entity';
import { CatalogItem } from '../../types/advanced';
import { tagsApi, categoriesApi, entityAssignmentsApi } from '../../api/advanced';
import { ApiError } from '../../api/client';
import { RequestGuard } from '../../utils/requestGuard';
import { TagIcon, CategoryIcon, PlusIcon, TrashIcon, RefreshIcon, CheckIcon } from '../Icons';

interface TagsCategoriesSectionProps {
  entity: Entity;
  onToast: (type: 'success' | 'error' | 'info', message: string, title?: string) => void;
}

export function TagsCategoriesSection({ entity, onToast }: TagsCategoriesSectionProps) {
  const formId = useId();
  const requestGuard = useRef(new RequestGuard()).current;
  const saveTagsGuard = useRef(new RequestGuard()).current;
  const saveCategoriesGuard = useRef(new RequestGuard()).current;
  // Catalogs
  const [allTags, setAllTags] = useState<CatalogItem[]>([]);
  const [allCategories, setAllCategories] = useState<CatalogItem[]>([]);

  // Entity assignments
  const [assignedTags, setAssignedTags] = useState<CatalogItem[]>([]);
  const [assignedCategories, setAssignedCategories] = useState<CatalogItem[]>([]);

  // Selected for editing
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([]);
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);

  // UI state
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isSavingTags, setIsSavingTags] = useState<boolean>(false);
  const [isSavingCategories, setIsSavingCategories] = useState<boolean>(false);
  const [newTagName, setNewTagName] = useState<string>('');
  const [newCategoryName, setNewCategoryName] = useState<string>('');
  const [isCreatingTag, setIsCreatingTag] = useState<boolean>(false);
  const [isCreatingCategory, setIsCreatingCategory] = useState<boolean>(false);
  const [tagError, setTagError] = useState<string | null>(null);
  const [categoryError, setCategoryError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    const request = requestGuard.begin();
    setIsLoading(true);
    setTagError(null);
    setCategoryError(null);
    try {
      const [tags, cats, eTags, eCats] = await Promise.all([
        tagsApi.getAll(),
        categoriesApi.getAll(),
        entityAssignmentsApi.getTags(entity.id),
        entityAssignmentsApi.getCategories(entity.id),
      ]);
      if (!requestGuard.isCurrent(request)) return;
      setAllTags(tags);
      setAllCategories(cats);
      setAssignedTags(eTags);
      setAssignedCategories(eCats);
      setSelectedTagIds(eTags.map((t) => t.id));
      setSelectedCategoryIds(eCats.map((c) => c.id));
    } catch (err) {
      if (!requestGuard.isCurrent(request)) return;
      const msg = err instanceof ApiError ? err.message : 'Failed to load tags and categories';
      setTagError(msg);
    } finally {
      if (requestGuard.isCurrent(request)) setIsLoading(false);
    }
  }, [entity.id, requestGuard]);

  useEffect(() => {
    loadData();
    return () => requestGuard.invalidate();
  }, [loadData, requestGuard]);

  useEffect(() => () => {
    saveTagsGuard.invalidate();
    saveCategoriesGuard.invalidate();
  }, [saveTagsGuard, saveCategoriesGuard]);

  // Tag Handlers
  const handleToggleTag = (tagId: string) => {
    setTagError(null);
    setSelectedTagIds((prev) => {
      if (prev.includes(tagId)) {
        return prev.filter((id) => id !== tagId);
      }
      if (prev.length >= 50) {
        setTagError('Maximum 50 tags per entity allowed.');
        return prev;
      }
      return [...prev, tagId];
    });
  };

  const handleCreateTag = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = newTagName.trim();
    if (!trimmed) return;

    setIsCreatingTag(true);
    setTagError(null);
    try {
      const created = await tagsApi.create(trimmed);
      setAllTags((prev) => [...prev, created]);
      setSelectedTagIds((prev) => {
        if (prev.length < 50 && !prev.includes(created.id)) {
          return [...prev, created.id];
        }
        return prev;
      });
      setNewTagName('');
      onToast('success', `Tag "${created.name}" created and selected.`, 'Tag Created');
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to create tag';
      setTagError(msg);
      onToast('error', msg, 'Tag Error');
    } finally {
      setIsCreatingTag(false);
    }
  };

  const handleDeleteCatalogTag = async (tag: CatalogItem) => {
    setTagError(null);
    try {
      await tagsApi.delete(tag.id);
      setAllTags((prev) => prev.filter((t) => t.id !== tag.id));
      setSelectedTagIds((prev) => prev.filter((id) => id !== tag.id));
      setAssignedTags((prev) => prev.filter((t) => t.id !== tag.id));
      onToast('success', `Tag "${tag.name}" deleted from catalog.`, 'Deleted');
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to delete tag';
      setTagError(msg);
      onToast('error', msg, 'Delete Conflict');
    }
  };

  const handleSaveTags = async () => {
    const request = saveTagsGuard.begin();
    setIsSavingTags(true);
    setTagError(null);
    try {
      const updated = await entityAssignmentsApi.replaceTags(entity.id, selectedTagIds);
      if (!saveTagsGuard.isCurrent(request)) return;
      setAssignedTags(updated);
      setSelectedTagIds(updated.map((t) => t.id));
      onToast('success', 'Assigned tags updated successfully.', 'Saved');
    } catch (err) {
      if (!saveTagsGuard.isCurrent(request)) return;
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to update tags';
      setTagError(msg);
      onToast('error', msg, 'Save Error');
    } finally {
      if (saveTagsGuard.isCurrent(request)) setIsSavingTags(false);
    }
  };

  // Category Handlers
  const handleToggleCategory = (catId: string) => {
    setCategoryError(null);
    setSelectedCategoryIds((prev) => {
      if (prev.includes(catId)) {
        return prev.filter((id) => id !== catId);
      }
      if (prev.length >= 20) {
        setCategoryError('Maximum 20 categories per entity allowed.');
        return prev;
      }
      return [...prev, catId];
    });
  };

  const handleCreateCategory = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = newCategoryName.trim();
    if (!trimmed) return;

    setIsCreatingCategory(true);
    setCategoryError(null);
    try {
      const created = await categoriesApi.create(trimmed);
      setAllCategories((prev) => [...prev, created]);
      setSelectedCategoryIds((prev) => {
        if (prev.length < 20 && !prev.includes(created.id)) {
          return [...prev, created.id];
        }
        return prev;
      });
      setNewCategoryName('');
      onToast('success', `Category "${created.name}" created and selected.`, 'Category Created');
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to create category';
      setCategoryError(msg);
      onToast('error', msg, 'Category Error');
    } finally {
      setIsCreatingCategory(false);
    }
  };

  const handleDeleteCatalogCategory = async (cat: CatalogItem) => {
    setCategoryError(null);
    try {
      await categoriesApi.delete(cat.id);
      setAllCategories((prev) => prev.filter((c) => c.id !== cat.id));
      setSelectedCategoryIds((prev) => prev.filter((id) => id !== cat.id));
      setAssignedCategories((prev) => prev.filter((c) => c.id !== cat.id));
      onToast('success', `Category "${cat.name}" deleted from catalog.`, 'Deleted');
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to delete category';
      setCategoryError(msg);
      onToast('error', msg, 'Delete Conflict');
    }
  };

  const handleSaveCategories = async () => {
    const request = saveCategoriesGuard.begin();
    setIsSavingCategories(true);
    setCategoryError(null);
    try {
      const updated = await entityAssignmentsApi.replaceCategories(entity.id, selectedCategoryIds);
      if (!saveCategoriesGuard.isCurrent(request)) return;
      setAssignedCategories(updated);
      setSelectedCategoryIds(updated.map((c) => c.id));
      onToast('success', 'Assigned categories updated successfully.', 'Saved');
    } catch (err) {
      if (!saveCategoriesGuard.isCurrent(request)) return;
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
          ? err.message
          : 'Failed to update categories';
      setCategoryError(msg);
      onToast('error', msg, 'Save Error');
    } finally {
      if (saveCategoriesGuard.isCurrent(request)) setIsSavingCategories(false);
    }
  };

  const hasTagChanges =
    selectedTagIds.length !== assignedTags.length ||
    selectedTagIds.some((id) => !assignedTags.some((t) => t.id === id));

  const hasCategoryChanges =
    selectedCategoryIds.length !== assignedCategories.length ||
    selectedCategoryIds.some((id) => !assignedCategories.some((c) => c.id === id));

  if (isLoading) {
    return (
      <div className="section-loading">
        <RefreshIcon size={18} className="animate-spin text-muted" />
        <span>Loading tags and categories...</span>
      </div>
    );
  }

  return (
    <div className="tags-categories-section">
      {/* Tags Section */}
      <div className="catalog-group">
        <div className="catalog-group-header">
          <div className="catalog-title-row">
            <TagIcon size={18} className="catalog-header-icon" />
            <h3 className="section-title">Tags</h3>
            <span className="badge-count">
              {selectedTagIds.length} / 50
            </span>
          </div>
          {hasTagChanges && (
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={handleSaveTags}
              disabled={isSavingTags}
            >
              <CheckIcon size={14} />
              <span>{isSavingTags ? 'Saving...' : 'Save Tags'}</span>
            </button>
          )}
        </div>

        {tagError && <div className="form-error-banner mb-3">{tagError}</div>}

        {/* Quick add new tag */}
        <form onSubmit={handleCreateTag} className="catalog-inline-form">
          <input
            id={`${formId}-new-tag`}
            type="text"
            className="form-input form-input-sm"
            placeholder="Create new tag (e.g. Electric Fleet)..."
            value={newTagName}
            maxLength={64}
            onChange={(e) => setNewTagName(e.target.value)}
          />
          <button
            type="submit"
            className="btn btn-secondary btn-sm"
            disabled={!newTagName.trim() || isCreatingTag}
          >
            <PlusIcon size={14} />
            <span>Add</span>
          </button>
        </form>

        {/* Tag chips selector */}
        <div className="catalog-chips-wrapper">
          {allTags.length === 0 ? (
            <p className="text-muted text-xs italic">
              No tags created yet. Type above to add your first tag.
            </p>
          ) : (
            allTags.map((tag) => {
              const isSelected = selectedTagIds.includes(tag.id);
              const isAssigned = assignedTags.some((t) => t.id === tag.id);

              return (
                <div
                  key={tag.id}
                  className={`catalog-chip ${isSelected ? 'chip-selected' : ''}`}
                >
                  <button
                    type="button"
                    className="chip-toggle-btn"
                    onClick={() => handleToggleTag(tag.id)}
                    aria-pressed={isSelected}
                  >
                    <span className="chip-indicator">{isSelected ? '✓' : '+'}</span>
                    <span className="chip-label">{tag.name}</span>
                  </button>
                  {!isAssigned && (
                    <button
                      type="button"
                      className="chip-delete-btn"
                      onClick={() => handleDeleteCatalogTag(tag)}
                      title={`Delete "${tag.name}" from catalog (not assigned to this entity)`}
                      aria-label={`Delete ${tag.name}`}
                    >
                      <TrashIcon size={12} />
                    </button>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>

      <hr className="section-divider" />

      {/* Categories Section */}
      <div className="catalog-group">
        <div className="catalog-group-header">
          <div className="catalog-title-row">
            <CategoryIcon size={18} className="catalog-header-icon" />
            <h3 className="section-title">Categories</h3>
            <span className="badge-count">
              {selectedCategoryIds.length} / 20
            </span>
          </div>
          {hasCategoryChanges && (
            <button
              type="button"
              className="btn btn-primary btn-sm"
              onClick={handleSaveCategories}
              disabled={isSavingCategories}
            >
              <CheckIcon size={14} />
              <span>{isSavingCategories ? 'Saving...' : 'Save Categories'}</span>
            </button>
          )}
        </div>

        {categoryError && <div className="form-error-banner mb-3">{categoryError}</div>}

        {/* Quick add new category */}
        <form onSubmit={handleCreateCategory} className="catalog-inline-form">
          <input
            id={`${formId}-new-category`}
            type="text"
            className="form-input form-input-sm"
            placeholder="Create new category (e.g. Critical Infrastructure)..."
            value={newCategoryName}
            maxLength={100}
            onChange={(e) => setNewCategoryName(e.target.value)}
          />
          <button
            type="submit"
            className="btn btn-secondary btn-sm"
            disabled={!newCategoryName.trim() || isCreatingCategory}
          >
            <PlusIcon size={14} />
            <span>Add</span>
          </button>
        </form>

        {/* Category chips selector */}
        <div className="catalog-chips-wrapper">
          {allCategories.length === 0 ? (
            <p className="text-muted text-xs italic">
              No categories created yet. Type above to add your first category.
            </p>
          ) : (
            allCategories.map((cat) => {
              const isSelected = selectedCategoryIds.includes(cat.id);
              const isAssigned = assignedCategories.some((c) => c.id === cat.id);

              return (
                <div
                  key={cat.id}
                  className={`catalog-chip ${isSelected ? 'chip-selected-cat' : ''}`}
                >
                  <button
                    type="button"
                    className="chip-toggle-btn"
                    onClick={() => handleToggleCategory(cat.id)}
                    aria-pressed={isSelected}
                  >
                    <span className="chip-indicator">{isSelected ? '✓' : '+'}</span>
                    <span className="chip-label">{cat.name}</span>
                  </button>
                  {!isAssigned && (
                    <button
                      type="button"
                      className="chip-delete-btn"
                      onClick={() => handleDeleteCatalogCategory(cat)}
                      title={`Delete "${cat.name}" from catalog (not assigned to this entity)`}
                      aria-label={`Delete ${cat.name}`}
                    >
                      <TrashIcon size={12} />
                    </button>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
