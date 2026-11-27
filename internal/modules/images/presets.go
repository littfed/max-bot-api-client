package images

var presetsCatalog = []Preset{
	{
		ID:                 "bg_replace",
		Mode:               ModeEdit,
		Title:              "Удалить / заменить фон",
		Description:        "Выделяет главный объект и помещает его на заданный студийный или сюжетный фон",
		PromptTemplate:     "Keep the main subject intact with clean edges, replace the background according to the user instruction, and match lighting and shadows naturally.",
		DefaultAspectRatio: "1:1",
		ExamplePrompt:      "Замени фон на уютную светлую кофейню с мягким утренним боке",
		ExampleBeforeURL:   "/presets/bg_replace_before.png",
		ExampleAfterURL:    "/presets/bg_replace_after.png",
	},
	{
		ID:                 "product_card",
		Mode:               ModeEdit,
		Title:              "Товарная карточка",
		Description:        "Превращает обычное фото товара в коммерческий кадр со студийным светом и тенями",
		PromptTemplate:     "Transform this product photo into a commercial e-commerce studio shot with clean podium or surface, soft studio key light, and realistic contact shadows while preserving product details.",
		DefaultAspectRatio: "1:1",
		ExamplePrompt:      "Поставь кроссовок на минималистичный бетонный подиум с мягким студийным светом",
		ExampleBeforeURL:   "/presets/product_card_before.png",
		ExampleAfterURL:    "/presets/product_card_after.png",
	},
	{
		ID:                 "retouch_light",
		Mode:               ModeEdit,
		Title:              "Ретушь и свет",
		Description:        "Выравнивает экспозицию, убирает шумы и улучшает цветокоррекцию без искажения деталей",
		PromptTemplate:     "Enhance lighting, balance exposure, remove digital noise, and apply natural color grading while strictly preserving the original composition and subject details.",
		DefaultAspectRatio: "4:3",
		ExamplePrompt:      "Сделай свет мягким и кинематографичным, убери шумы и выровняй баланс белого",
		ExampleBeforeURL:   "/presets/retouch_light_before.png",
		ExampleAfterURL:    "/presets/retouch_light_after.png",
	},
	{
		ID:                 "object_remove",
		Mode:               ModeEdit,
		Title:              "Удаление объекта",
		Description:        "Убирает лишний объект, человека или надпись по текстовой инструкции",
		PromptTemplate:     "Cleanly remove the specified unwanted object, person, or text from the image and seamlessly inpaint the background to match surrounding textures.",
		DefaultAspectRatio: "4:3",
		ExamplePrompt:      "Убери прохожих и дорожный знак на заднем плане",
		ExampleBeforeURL:   "/presets/object_remove_before.png",
		ExampleAfterURL:    "/presets/object_remove_after.png",
	},
	{
		ID:                 "style_transfer",
		Mode:               ModeEdit,
		Title:              "Стилизация фото",
		Description:        "Переносит снимок в выбранный художественный стиль (3D, аниме, акварель, ретро)",
		PromptTemplate:     "Restyle the entire photo into the requested artistic medium while keeping the original pose, framing, and recognizable subject identity.",
		DefaultAspectRatio: "1:1",
		ExamplePrompt:      "Преврати портрет в стильную 3D-иллюстрацию в духе Pixar с тёплым светом",
		ExampleBeforeURL:   "/presets/style_transfer_before.png",
		ExampleAfterURL:    "/presets/style_transfer_after.png",
	},
	{
		ID:                 "social_poster",
		Mode:               ModeGenerate,
		Title:              "Обложка для поста",
		Description:        "Создаёт чистую яркую композицию для канала, статьи или анонса",
		PromptTemplate:     "Create a clean, vibrant editorial cover illustration suitable for a social media post or article banner with clear focal point and balanced negative space.",
		DefaultAspectRatio: "16:9",
		ExamplePrompt:      "Футуристичный минималистичный постер для анонса AI-обновления в тёмно-синих и неоновых тонах",
		ExampleAfterURL:    "/presets/social_poster_after.png",
	},
	{
		ID:                 "brand_logo",
		Mode:               ModeGenerate,
		Title:              "Логотип / иконка",
		Description:        "Генерирует минималистичную эмблему или знак бренда на чистом фоне",
		PromptTemplate:     "Design a minimalist, memorable vector-style brand emblem or app icon centered on a clean solid background, crisp geometry, no clutter.",
		DefaultAspectRatio: "1:1",
		ExamplePrompt:      "Минималистичный геометрический логотип кофейни в виде зёрнышка и волны на кремовом фоне",
		ExampleAfterURL:    "/presets/brand_logo_after.png",
	},
	{
		ID:                 "sticker_pack",
		Mode:               ModeGenerate,
		Title:              "Стикер / персонаж",
		Description:        "Генерирует выразительного персонажа с чётким контуром на однотонном фоне",
		PromptTemplate:     "Create an expressive mascot sticker character with bold clean outline, vibrant shading, and a solid uniform background ready for cutout.",
		DefaultAspectRatio: "1:1",
		ExamplePrompt:      "Милый рыжий кот-разработчик в очках с кружкой кофе, стиль стикера с белым контуром",
		ExampleAfterURL:    "/presets/sticker_pack_after.png",
	},
}

// ListPresets returns a copy of all available image studio presets.
func ListPresets() []Preset {
	out := make([]Preset, len(presetsCatalog))
	copy(out, presetsCatalog)
	return out
}

// GetPreset returns a preset by ID.
func GetPreset(id string) (Preset, bool) {
	for _, p := range presetsCatalog {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}
