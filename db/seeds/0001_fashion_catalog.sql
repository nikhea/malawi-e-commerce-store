-- 0001_fashion_catalog.sql
-- Demo catalogue for the Malawi Store female-fashion storefront.
--
-- Scope: 5 root categories, 50 products, size variants for every product, and
-- opening stock for every variant. Product and category image columns are left
-- empty so real Cloudinary assets can be attached later without this seed
-- overwriting them.
--
-- Prices below are written in MWK tambala (the original source list) and
-- converted to USD cents at 1 USD = 1,700 MWK when inserted. The store
-- stores USD cents; editing a price here means editing the MWK source.
--
-- Apply with:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/seeds/0001_fashion_catalog.sql
--
-- Safe to re-run: categories, products, and variants upsert by slug/SKU.
-- Existing image columns and inventory counts are preserved.

BEGIN;

CREATE TEMP TABLE seed_fashion_products (
    category_slug TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    slug          TEXT        NOT NULL UNIQUE,
    description   TEXT        NOT NULL,
    price_cents   BIGINT      NOT NULL CHECK (price_cents >= 0),
    sizes         TEXT[]      NOT NULL,
    on_hand_qty   INTEGER     NOT NULL CHECK (on_hand_qty >= 0)
) ON COMMIT DROP;

INSERT INTO seed_fashion_products
    (category_slug, name, slug, description, price_cents, sizes, on_hand_qty)
VALUES
    -- Dresses
    ('dresses', 'Chitenje Wrap Midi Dress', 'chitenje-wrap-midi-dress',
     'Wrap-front midi in soft chitenje-print cotton with a self-tie waist, flutter sleeves, and side pocket. Fully lined bodice; machine wash cold, line dry.',
     14500000, ARRAY['XS','S','M','L','XL']::TEXT[], 16),
    ('dresses', 'Lilongwe Linen Shirt Dress', 'lilongwe-linen-shirt-dress',
     'Relaxed shirt dress in breathable linen-cotton with a concealed button placket, removable belt, and below-knee hem. Side pockets; cool iron.',
     12500000, ARRAY['XS','S','M','L','XL']::TEXT[], 20),
    ('dresses', 'Mulanje Tiered Maxi Dress', 'mulanje-tiered-maxi-dress',
     'Three-tier cotton voile maxi with a smocked bodice, adjustable straps, and pockets. Lightweight and fully lined skirt; hand wash cold.',
     16500000, ARRAY['XS','S','M','L','XL']::TEXT[], 12),
    ('dresses', 'Nkhotakota Satin Slip Dress', 'nkhotakota-satin-slip-dress',
     'Bias-cut satin slip with a cowl neck, adjustable straps, and midi hem. Fully lined; dry clean recommended.',
     13500000, ARRAY['XS','S','M','L','XL']::TEXT[], 14),
    ('dresses', 'Zomba Ribbed Bodycon Dress', 'zomba-ribbed-bodycon-dress',
     'Stretch-rib knit bodycon with a square neck, cap sleeves, and midi length. Snug fit; machine wash cold inside out.',
     9500000, ARRAY['XS','S','M','L','XL']::TEXT[], 24),
    ('dresses', 'Cape Maclear Beach Sundress', 'cape-maclear-beach-sundress',
     'Airy cotton sundress with a tie-back bodice, smocked waist, and tiered skirt. Pack-friendly; machine wash gentle.',
     8500000, ARRAY['XS','S','M','L','XL']::TEXT[], 28),
    ('dresses', 'Blantyre Puff-Sleeve Midi Dress', 'blantyre-puff-sleeve-midi-dress',
     'Structured cotton-poplin midi with puff sleeves, a fitted bodice, and full skirt. Concealed zip; dry clean or gentle hand wash.',
     11500000, ARRAY['XS','S','M','L','XL']::TEXT[], 18),
    ('dresses', 'Kasungu Belted Shirt Dress', 'kasungu-belted-shirt-dress',
     'Utility shirt dress in stretch cotton twill with a self-belt, chest pockets, and roll-tab sleeves. Machine wash cold; tumble low.',
     13500000, ARRAY['XS','S','M','L','XL']::TEXT[], 16),
    ('dresses', 'Mzuzu Knit Sweater Dress', 'mzuzu-knit-sweater-dress',
     'Fine-gauge knit sweater dress with a mock neck, raglan seams, and ribbed cuffs. Midweight viscose blend; hand wash and dry flat.',
     12000000, ARRAY['XS','S','M','L','XL']::TEXT[], 15),
    ('dresses', 'Nyika Velvet Evening Gown', 'nyika-velvet-evening-gown',
     'Floor-length stretch-velvet gown with a sweetheart neckline, gathered waist, and side slit. Fully lined; dry clean only.',
     19500000, ARRAY['XS','S','M','L','XL']::TEXT[], 8),

    -- Tops and blouses
    ('tops-blouses', 'Chichewa Ruffle Blouse', 'chichewa-ruffle-blouse',
     'Cotton blouse with a high ruffle neck, covered buttons, and softly gathered sleeves. Relaxed fit; machine wash cold, line dry.',
     4500000, ARRAY['XS','S','M','L','XL']::TEXT[], 22),
    ('tops-blouses', 'Nsanje Cotton Poplin Shirt', 'nsanje-cotton-poplin-shirt',
     'Crisp cotton-poplin shirt with a classic collar, curved hem, and chest pocket. Tuck or knot it over trousers; machine wash.',
     5500000, ARRAY['XS','S','M','L','XL']::TEXT[], 20),
    ('tops-blouses', 'Mzimba Cropped Linen Top', 'mzimba-cropped-linen-top',
     'Cropped linen-blend top with a square neck, wide straps, and back zip. Pair with high-rise bottoms; hand wash cold.',
     3900000, ARRAY['XS','S','M','L','XL']::TEXT[], 26),
    ('tops-blouses', 'Salima Satin Cami', 'salima-satin-cami',
     'Adjustable-strap satin camisole with a V neckline and straight hem. Layer under a blazer or wear alone; hand wash.',
     3500000, ARRAY['XS','S','M','L','XL']::TEXT[], 30),
    ('tops-blouses', 'Dedza Puff-Sleeve Peplum Top', 'dedza-puff-sleeve-peplum-top',
     'Stretch-cotton peplum top with voluminous puff sleeves and a defined waist. Concealed back zip; machine wash gentle.',
     4900000, ARRAY['XS','S','M','L','XL']::TEXT[], 18),
    ('tops-blouses', 'Thyolo Oversized Poplin Shirt', 'thyolo-oversized-poplin-shirt',
     'Oversized poplin shirt with dropped shoulders, a patch pocket, and side slits. Wear as a shirt or light cover-up; machine wash.',
     5900000, ARRAY['XS','S','M','L','XL']::TEXT[], 20),
    ('tops-blouses', 'Mangochi Smocked Bardot Top', 'mangochi-smocked-bardot-top',
     'Off-shoulder smocked top with a ruffled neckline and cropped length. Stretch fit; hand wash and line dry.',
     4200000, ARRAY['XS','S','M','L','XL']::TEXT[], 24),
    ('tops-blouses', 'Ntcheu Ribbed Tank', 'ntcheu-ribbed-tank',
     'Everyday ribbed tank with a scoop neck and slim straps. Four-way stretch cotton blend; machine wash cold.',
     2500000, ARRAY['XS','S','M','L','XL']::TEXT[], 36),
    ('tops-blouses', 'Karonga Tie-Front Blouse', 'karonga-tie-front-blouse',
     'Satin-touch blouse with a plunging tie front, gathered shoulders, and a relaxed hem. Hand wash cold; cool iron.',
     4700000, ARRAY['XS','S','M','L','XL']::TEXT[], 22),
    ('tops-blouses', 'Balaka Embroidered Tunic', 'balaka-embroidered-tunic',
     'Cotton tunic with tonal embroidery at the neckline, three-quarter sleeves, and side vents. Easy fit; machine wash gentle.',
     6500000, ARRAY['XS','S','M','L','XL']::TEXT[], 16),

    -- Bottoms
    ('bottoms', 'Chikwawa Wide-Leg Trousers', 'chikwawa-wide-leg-trousers',
     'High-waist wide-leg trousers in fluid crepe with a flat front, side pockets, and pressed pleats. Concealed zip; dry clean.',
     7500000, ARRAY['6','8','10','12','14']::TEXT[], 18),
    ('bottoms', 'Neno High-Rise Straight Jeans', 'neno-high-rise-straight-jeans',
     'Rigid cotton-denim jeans with a high rise, straight leg, and five-pocket construction. Button fly; machine wash cold.',
     8500000, ARRAY['6','8','10','12','14']::TEXT[], 20),
    ('bottoms', 'Machinga Pleated Midi Skirt', 'machinga-pleated-midi-skirt',
     'Knife-pleated midi skirt with an elastic waist and satin finish. Falls below the knee; hand wash or dry clean.',
     6500000, ARRAY['6','8','10','12','14']::TEXT[], 20),
    ('bottoms', 'Phalombe Tailored Shorts', 'phalombe-tailored-shorts',
     'High-waist tailored shorts in stretch suiting fabric with front pleats and slant pockets. Above-knee length; dry clean.',
     4500000, ARRAY['6','8','10','12','14']::TEXT[], 24),
    ('bottoms', 'Rumphi Cargo Pants', 'rumphi-cargo-pants',
     'Relaxed cargo pants in cotton twill with bellow pockets, drawcord hem, and a high waist. Machine wash cold.',
     7200000, ARRAY['6','8','10','12','14']::TEXT[], 16),
    ('bottoms', 'Ntchisi A-Line Denim Skirt', 'ntchisi-a-line-denim-skirt',
     'A-line denim skirt with a high waist, front button placket, and patch pockets. Rigid denim; machine wash cold.',
     5800000, ARRAY['6','8','10','12','14']::TEXT[], 22),
    ('bottoms', 'Likoma Linen Wide-Leg Pants', 'likoma-linen-wide-leg-pants',
     'Breathable linen-blend pants with a drawstring waist, wide leg, and side pockets. Relaxed fit; machine wash gentle.',
     6900000, ARRAY['6','8','10','12','14']::TEXT[], 20),
    ('bottoms', 'Chiradzulu Paperbag Waist Trousers', 'chiradzulu-paperbag-waist-trousers',
     'Paperbag-waist trousers with a removable tie belt, tapered leg, and pleated front. Stretch blend; dry clean.',
     7000000, ARRAY['6','8','10','12','14']::TEXT[], 17),
    ('bottoms', 'Mwanza Satin Bias Skirt', 'mwanza-satin-bias-skirt',
     'Bias-cut satin midi skirt with a hidden side zip and fluid drape. Fully lined; dry clean recommended.',
     6200000, ARRAY['6','8','10','12','14']::TEXT[], 19),
    ('bottoms', 'Nkhata Bay Utility Joggers', 'nkhata-bay-utility-joggers',
     'Soft cotton-blend joggers with utility pockets, an elastic drawcord waist, and tapered cuffs. Machine wash cold.',
     5200000, ARRAY['6','8','10','12','14']::TEXT[], 24),

    -- Outerwear
    ('outerwear', 'Mulanje Denim Jacket', 'mulanje-denim-jacket',
     'Classic denim jacket with a button front, chest flap pockets, and adjustable waist tabs. Midweight cotton; machine wash cold.',
     11500000, ARRAY['XS','S','M','L','XL']::TEXT[], 16),
    ('outerwear', 'Zomba Tailored Blazer', 'zomba-tailored-blazer',
     'Single-breasted blazer with structured shoulders, a notch lapel, and welt pockets. Fully lined; dry clean.',
     16500000, ARRAY['XS','S','M','L','XL']::TEXT[], 12),
    ('outerwear', 'Nyika Wool-Blend Coat', 'nyika-wool-blend-coat',
     'Longline wool-blend coat with a double-breasted front, belted waist, and back vent. Fully lined; dry clean only.',
     24500000, ARRAY['XS','S','M','L','XL']::TEXT[], 8),
    ('outerwear', 'Chitenje Reversible Jacket', 'chitenje-reversible-jacket',
     'Reversible quilted jacket with a chitenje print on one side and solid cotton on the other. Lightly padded; machine wash gentle.',
     13500000, ARRAY['XS','S','M','L','XL']::TEXT[], 14),
    ('outerwear', 'Cape Maclear Cropped Cardigan', 'cape-maclear-cropped-cardigan',
     'Cropped fine-knit cardigan with a V neck, shell buttons, and ribbed trim. Lightweight cotton blend; hand wash.',
     7500000, ARRAY['XS','S','M','L','XL']::TEXT[], 20),
    ('outerwear', 'Lilongwe Trench Coat', 'lilongwe-trench-coat',
     'Double-breasted trench in water-resistant cotton gabardine with epaulettes, storm flap, and belt. Dry clean.',
     22500000, ARRAY['XS','S','M','L','XL']::TEXT[], 9),
    ('outerwear', 'Kasungu Quilted Jacket', 'kasungu-quilted-jacket',
     'Diamond-quilted jacket with a funnel neck, snap front, and zip pockets. Lightly insulated; machine wash cold.',
     14500000, ARRAY['XS','S','M','L','XL']::TEXT[], 15),
    ('outerwear', 'Blantyre Satin Bomber', 'blantyre-satin-bomber',
     'Satin bomber with ribbed collar, cuffs, and hem, plus a two-way zip. Fully lined; dry clean.',
     12500000, ARRAY['XS','S','M','L','XL']::TEXT[], 13),
    ('outerwear', 'Mzuzu Oversized Shirt Jacket', 'mzuzu-oversized-shirt-jacket',
     'Oversized shirt jacket in brushed cotton with a camp collar, patch pockets, and side slits. Machine wash cold.',
     10500000, ARRAY['XS','S','M','L','XL']::TEXT[], 18),
    ('outerwear', 'Nkhotakota Faux Leather Jacket', 'nkhotakota-faux-leather-jacket',
     'Cropped faux-leather jacket with a zip front, lapels, and quilted lining. Wipe clean; do not machine wash.',
     15500000, ARRAY['XS','S','M','L','XL']::TEXT[], 11),

    -- Accessories
    ('accessories', 'Chitenje Print Scarf', 'chitenje-print-scarf',
     'Lightweight cotton scarf in a bold chitenje print with hand-rolled ends. 180 x 70 cm; hand wash cold.',
     2200000, ARRAY['One Size']::TEXT[], 40),
    ('accessories', 'Lilongwe Leather Tote', 'lilongwe-leather-tote',
     'Full-grain leather tote with a zip top, cotton lining, and interior slip pocket. Fits a 13-inch laptop; wipe clean.',
     8500000, ARRAY['One Size']::TEXT[], 12),
    ('accessories', 'Zomba Beaded Hoop Earrings', 'zomba-beaded-hoop-earrings',
     'Gold-tone hoop earrings with hand-beaded accents and hypoallergenic posts. 4 cm drop; wipe with a soft cloth.',
     1800000, ARRAY['One Size']::TEXT[], 50),
    ('accessories', 'Cape Maclear Straw Hat', 'cape-maclear-straw-hat',
     'Wide-brim woven straw hat with an adjustable inner band and UPF 50+ protection. Spot clean only.',
     3500000, ARRAY['One Size']::TEXT[], 25),
    ('accessories', 'Mulanje Woven Belt', 'mulanje-woven-belt',
     'Hand-woven leather belt with a brushed brass buckle and five-hole adjustment. 3 cm wide; spot clean.',
     2800000, ARRAY['S','M','L']::TEXT[], 20),
    ('accessories', 'Blantyre Mini Crossbody Bag', 'blantyre-mini-crossbody-bag',
     'Structured mini crossbody in pebbled vegan leather with a magnetic flap and adjustable strap. Wipe clean.',
     5500000, ARRAY['One Size']::TEXT[], 18),
    ('accessories', 'Nkhotakota Silk Hair Wrap', 'nkhotakota-silk-hair-wrap',
     'Mulberry-silk hair wrap in a hand-painted print with a pre-tied knot. 100 x 20 cm; hand wash cold.',
     2500000, ARRAY['One Size']::TEXT[], 35),
    ('accessories', 'Kasungu Layered Necklace Set', 'kasungu-layered-necklace-set',
     'Three-layer gold-tone necklace set with a coin pendant and extender chain. Nickel-free; wipe clean.',
     3200000, ARRAY['One Size']::TEXT[], 30),
    ('accessories', 'Mzuzu Canvas Weekender', 'mzuzu-canvas-weekender',
     'Heavyweight canvas weekender with leather handles, a detachable strap, and a zip interior pocket. Spot clean.',
     7200000, ARRAY['One Size']::TEXT[], 14),
    ('accessories', 'Salima Statement Clutch', 'salima-statement-clutch',
     'Structured clutch in metallic faux leather with a magnetic clasp and optional chain strap. Wipe clean.',
     4800000, ARRAY['One Size']::TEXT[], 16);

INSERT INTO categories (name, slug, description)
VALUES
    ('Dresses', 'dresses',
     'Day dresses, workwear, church outfits, and evening gowns cut for warm-weather comfort.'),
    ('Tops & Blouses', 'tops-blouses',
     'Shirts, blouses, camisoles, and knit tops in breathable cotton, linen, and satin.'),
    ('Bottoms', 'bottoms',
     'Trousers, jeans, skirts, and shorts with flattering fits for everyday and occasion wear.'),
    ('Outerwear', 'outerwear',
     'Jackets, blazers, coats, and cardigans for cool evenings and rainy-season layering.'),
    ('Accessories', 'accessories',
     'Bags, belts, scarves, jewellery, and hats that finish a look.')
ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    updated_at = now();

INSERT INTO products
    (category_id, name, slug, description, price_cents, currency, is_active)
SELECT c.id,
       s.name,
       s.slug,
       s.description,
       ROUND(s.price_cents / 1700.0)::BIGINT,
       'USD',
       TRUE
FROM seed_fashion_products s
JOIN categories c ON c.slug = s.category_slug
ON CONFLICT (slug) DO UPDATE
SET category_id = EXCLUDED.category_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    price_cents = EXCLUDED.price_cents,
    currency = EXCLUDED.currency,
    is_active = EXCLUDED.is_active,
    updated_at = now();

INSERT INTO variants (product_id, name, sku, price_cents, is_active)
SELECT p.id,
       size,
       p.slug || '-' || regexp_replace(lower(size), '[^a-z0-9]+', '-', 'g'),
       p.price_cents,
       TRUE
FROM seed_fashion_products s
JOIN products p ON p.slug = s.slug
CROSS JOIN LATERAL unnest(s.sizes) AS size
ON CONFLICT (sku) DO UPDATE
SET name = EXCLUDED.name,
    price_cents = EXCLUDED.price_cents,
    is_active = EXCLUDED.is_active,
    updated_at = now();

INSERT INTO inventory (product_id, variant_id, on_hand_qty, reserved_qty)
SELECT v.product_id,
       v.id,
       s.on_hand_qty,
       0
FROM seed_fashion_products s
JOIN products p ON p.slug = s.slug
JOIN variants v ON v.product_id = p.id
ON CONFLICT (product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'))
DO NOTHING;

COMMIT;
