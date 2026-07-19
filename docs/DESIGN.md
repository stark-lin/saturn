# SATURN Design

This document describes the design principles for the SATURN frontend interface and component reference pages. The currently accompanying HTML prototype can be used as `docs/DESIGN.html`, and this document serves as `docs/DESIGN.md`.

## 1. Design Positioning

SATURN's interface is neither a traditional SaaS template nor a purely decorative mockup. Its positioning is:

> A self-hosted console for personal data management, visually approaching a low-saturation, hard-bordered, retro-futuristic system panel.

Design goals:

* Clearly express the product temperament of a "personal data console".
* Use stable modular navigation to support long-term usage scenarios.
* Reinforce the system feel using `ref_code`, monospace numbering, status lights, and instrument controls.
* Keep components implementable, maintainable, and transferable to real frontend code.
* Avoid high-gloss glass, strong shadows, excessive skeuomorphism, and the ordinary templated SaaS feel.

## 2. Core Visual Language

### 2.1 Keywords

* Low-saturation
* Hard border
* Matte surface
* Fixed chassis
* Data console
* Instrument panel
* Reference-oriented
* Self-hosted system

### 2.2 Visual Rules

* Use a low-contrast dot grid for the background to provide an engineering blueprint feel.
* Use hard borders for cards and controls, not relying on drop shadows to create hierarchy.
* Fixed areas should be dimmer and heavier than content areas, not brighter.
* Interaction feedback should prioritize state changes, avoiding excessive depression animations.
* Orange is used for main emphasis and 'on' states; green is used for normal status, selection indication, and system operation.
* Monospace fonts are exclusively used for branding, numbering, statuses, labels, times, and system readouts.

## 3. Typography, Density, and Layout Contracts

SATURN's frontend styles must use `Design Token + Component Contract`. Pages do not achieve consistency through scattered manual tweaks, but rather by defining fonts, font sizes, spacing, control heights, and fixed layouts first, then reusing these constraints via components.

Core principle:

> Monospace for system/data, Sans-serif for reading/labels.

### 3.1 Font Families

Fonts are divided into only two categories:

| Purpose | Font |
| --- | --- |
| Body text, descriptions, form labels, note content, Markdown preview | Sans-serif |
| `ref_code`, IDs, amounts, dates, times, statuses, commands, module shortcodes, system readouts | Monospace |

Recommended tokens:

```css
:root {
  --font-sans: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --font-mono: "IBM Plex Mono", "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}
```

Rules:

* `body` defaults to using `--font-sans`.
* System identity, numbering, statuses, machine data in tables, and short button text can use `--font-mono`.
* Do not set the entire application to a site-wide monospace font; long-text reading areas must prioritize readability.
* Do not introduce decorative fonts, serif fonts, or page-level temporary font stacks.

### 3.2 Type Scale

SATURN only allows using the following font size tokens:

```css
:root {
  --text-xs: 11px;
  --text-sm: 12px;
  --text-md: 14px;
  --text-lg: 16px;
  --text-xl: 20px;
  --text-2xl: 24px;
  --text-3xl: 32px;

  --line-tight: 1.15;
  --line-normal: 1.45;
  --line-loose: 1.65;
}
```

Font size semantics:

| Scenario | Size | Font | Rule |
| --- | --- | --- | --- |
| Page titles | `--text-xl` / `--text-2xl` | Sans | Only used for top titles on module pages |
| Module hero or component reference page large title | `--text-3xl` | Sans or Mono | Maximum of one per page |
| Card titles / section titles | `--text-sm` | Mono | uppercase, short text |
| Form labels / metadata labels | `--text-xs` | Mono | uppercase, short text |
| Body text / descriptions / form body | `--text-md` | Sans | Default reading size |
| Secondary descriptions | `--text-sm` | Sans | Does not carry critical data |
| Data values / IDs / Dates / Amounts | `--text-sm` / `--text-md` | Mono | Requires alignment and scanning |
| Buttons | `--text-xs` | Mono | uppercase, prioritize single command words |

Typography rules:

* Do not write temporary font sizes directly like `13px`, `15px`, `17px`.
* Use `--line-normal` for body text; large blocks of Markdown preview can use `--line-loose`.
* Titles and system readouts can use `--line-tight`.
* Normal reading text has a `letter-spacing` of `0`; only uppercase mono labels / buttons / statuses can use `0.06em` to `0.12em`.
* Do not use negative letter spacing.

### 3.3 Spacing Scale

Spacing only uses four levels: `8px / 16px / 24px / 32px`. The existing `--space-*` names are retained for component reuse and migration convenience, but these names can only map to the four-level spacing values.

```css
:root {
  --space-0: 0;
  --space-1: 8px;
  --space-2: 8px;
  --space-3: 16px;
  --space-4: 16px;
  --space-5: 24px;
  --space-6: 24px;
  --space-7: 32px;
  --space-8: 32px;
  --space-10: 32px;
  --space-12: 32px;
}
```

Recommended density:

| Scenario | Spacing |
| --- | --- |
| Between label and value | `--space-1` / `--space-2` |
| Between fields in the same group | `--space-2` |
| Vertical spacing between form fields | `--space-3` / `--space-4` |
| panel / card / section surface padding | `--space-4` |
| Sibling panel gap | `--space-4` |
| Padding of boxes within the main page area | `--space-4` |
| Gap between modules in the main View | `--space-4` only |
| Large gap within a module | `--space-6` / `--space-7` / `--space-8`, only when structurally necessary |
| Table header height | `32px` |
| Table row height | `40px` |
| Standard button height | `32px` |

Rules:

* Do not randomly write temporary values like `margin: 13px`, `padding: 18px`, `gap: 20px` within components.
* Page and component `padding`, `margin`, `gap`, `top` / `left` whitespace layouts must use spacing tokens or be calculated by combining spacing tokens.
* Bordered content boxes, including panel, card, stat, form, detail, empty state, and preview, use `--space-4` (16px) uniformly for internal padding.
* The gap between modules in the main View is always `--space-4` (16px). It must not increase based on page type, module level, or visual weight.
* Special visual lines, borders, and focus rings can use non-spacing token values like `1px` or `2px`.
* Control heights, icon sizes, border radii, decorative dots, and illustration sizes do not belong to the spacing scale but should be managed by their respective semantic tokens.
* Horizontal and vertical page whitespace is controlled by semantic tokens and is not manually adjusted repeatedly in each module.

### 3.4 Component Density Contracts

Components must first adhere to density contracts before module variations.

```css
:root {
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;

  --control-h-sm: 28px;
  --control-h-md: 32px;
  --control-h-lg: 40px;

  --page-padding: var(--space-4);
  --panel-padding: var(--space-4);
  --panel-gap: var(--space-4);
  --section-gap: var(--space-4);
  --module-gap: var(--space-4);
  --layout-gap: var(--module-gap);
  --field-gap: var(--space-2);
  --field-stack-gap: var(--space-4);

  --table-head-h: 32px;
  --table-row-h: 40px;
  --button-h: var(--control-h-md);
}
```

Component rules:

* Button: Use `--button-h` for height, `--space-3` for horizontal padding, `--text-xs` for font size, short verbs uppercase.
* Input / Select: Use `--control-h-md` for height, `--space-3` for horizontal padding, `--text-sm` or `--text-md` for font size.
* Panel / Card / Stat / Detail: padding uses `--panel-padding` or `--page-padding`, both mapping to `--space-4` (16px). These surface types do not justify nesting cards inside cards.
* Table: header height uses `--table-head-h`, row height uses `--table-row-h`; headers use mono uppercase, data columns choose sans or mono based on data type.
* Status / Chip: Used for short metadata, not to carry long sentences; height and padding must be stable and cannot jump in layout due to content.

### 3.5 Module-Level Breathing Room

SATURN borrows only the sense of breathing room that the Carbon Design System creates through module layout. It does not copy Carbon's component styling, visual language, or large-spacing system.

SATURN does not rely on large areas of whitespace. Its breathing room comes from clear module boundaries, a stable main View structure, and a deliberately limited number of modules.

#### 3.5.1 Module Gaps Are Always 16px

Use a uniform `16px` gap between modules. Do not increase it based on page type, module level, or visual weight.

```text
Module A

16px

Module B

16px

Module C
```

Create breathing room by:

* Limiting the number of modules visible in one screen.
* Keeping each module's internal structure simple.
* Establishing a clear primary-secondary hierarchy.
* Avoiding excessive nesting.
* Separating regions through alignment, boundaries, and background hierarchy.

Do not create breathing room by increasing module gaps.

#### 3.5.2 The Main View Contains Only 1, 2, or 4 Modules

The main View may contain exactly:

```text
1 module
2 modules
4 modules
```

Three modules must never be placed side by side. Restricting the module count gives the main interface a stable, predictable rhythm and prevents the width imbalance, dispersed visual focus, and responsive degradation caused by three-column layouts.

Allowed layouts:

```text
1 module

|            Primary module            |
```

```text
2 modules

|       Module A       |       Module B       |
```

```text
4 modules

|     Module A     |     Module B     |
|     Module C     |     Module D     |
```

Prohibited layout:

```text
|   Module A   |   Module B   |   Module C   |
```

#### 3.5.3 Each Module Has One Primary Layout Direction

Every module must have one clear primary structure. Typical structures are:

```text
Module title
Module description or actions
Module body
```

or:

```text
Left-side labels / navigation
Right-side main content
```

Do not combine several complex layouts inside one module, such as:

* Multiple summary cards across the top.
* A separate panel on the left.
* Several parallel regions on the right.
* More nested cards at the bottom.
* Another card grid inside a card.

A module should be close to:

```text
Section header
----------------------------------------
Main content
```

not:

```text
Card
  Card
    Card
```

#### 3.5.4 Do Not Nest Cards Indefinitely

Express SATURN's module boundaries through the overall layout, not by placing cards inside cards.

Modules may be distinguished through:

* The fixed `16px` gap.
* Dividers.
* Background hierarchy.
* Heading hierarchy.
* Grid alignment.
* Changes in content density.

Do not add a separate card container around every content group unless a clear interaction or semantic requirement calls for it.

#### 3.5.5 Breathing Room Comes from Quantity Control, Not Enlarged Whitespace

SATURN may maintain a relatively high information density. Tables, fields, buttons, and action areas may be compact, but the main View must not contain too many modules competing for attention.

```text
Breathing room
= fixed 16px module gaps
+ controlled module count
+ clear primary-secondary hierarchy
+ one internal structure per module
- three modules side by side
- multi-level card nesting
- meaningless large-area padding
```

#### 3.5.6 Recommended Main View Skeletons

##### Single-Module View

Use for object details, editors, tables, or a primary workspace.

```text
Page title area

16px

Primary module
```

##### Two-Module View

Use when primary and supporting content need to appear side by side.

```text
Page title area

16px

| Primary module | Supporting module |
```

The modules may have equal widths or a clear primary-secondary ratio, but the layout must not be split into a third column.

##### Four-Module View

Use for status overviews, system control areas, or a fixed four-quadrant layout.

```text
Page title area

16px

| Module A | Module B |

16px

| Module C | Module D |
```

Keep the complete `2 x 2` structure. Do not degrade it into three modules on one row and one module on the next.

#### 3.5.7 Evaluation Criteria

Blur the page content and inspect only the module boundaries and large-scale structure.

A compliant page makes all of the following immediately recognizable:

* The main View.
* The current primary module.
* Supporting modules.
* The relationships between modules.
* The entry point for page actions.

If the blurred page contains many similarly sized rectangles at the same hierarchy level, the page has too many modules or its module boundaries are out of control.

#### 3.5.8 Module Page Contracts

Module pages use a small number of stable layout contracts:

| Layout | Purpose |
| --- | --- |
| `PageShell` | Root container for all module pages |
| `MainView` | Contains exactly 1, 2, or 4 modules |
| `MainSplit` | Two-module layout with a primary workspace and supporting detail or metadata |
| `QuadView` | Complete `2 x 2` four-module layout |
| `ObjectList` | Ledgers, files, calendar events, or note lists within a module |
| `DetailView` | Immutable object details within a module |
| `ActionBar` | `RETURN` / `CREATE` / `SAVE` / `DELETE` / `VOID` within the relevant module |
| `PaginationBar` | List pagination within a module |

Recommended structural rules:

```css
.ui-page {
  padding: var(--page-padding);
}

.ui-main-view {
  display: grid;
  gap: var(--module-gap);
}

.ui-main-view--two {
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr);
}

.ui-main-view--four {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.ui-bottom-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--space-3);
}
```

Rules:

* The title area is followed by exactly `16px` before the main View.
* The main View contains one, two, or four modules and uses a `16px` gap throughout.
* A two-module layout may be equal-width or use a clear primary-secondary ratio, but it cannot gain a third column.
* A four-module layout remains a complete `2 x 2` grid.
* Page-level actions belong to the relevant module and should be placed at the bottom right of its content area when practical. Dangerous and regular operations maintain clear separation.
* List pagination uses a unified `PaginationBar` within the relevant module rather than a separate competing page-level module.
* Mobile layouts may collapse to one column, but module order, hierarchy, font sizes, control heights, and spacing tokens remain stable.

In summary:

> SATURN modules should be compact and direct like engineering tools. A fixed 16px gap establishes rhythm between modules. The main View permits only 1, 2, or 4 modules, and three modules may never be placed side by side.

## 4. Layout Model

SATURN's main structure is divided into three layers:

```text
Fixed System Layer
├── SATURN Top Banner
└── Control Rack

Scrollable Content Layer
└── Module content / component references

Background Layer
└── Dot grid / matte system background
```

The dot matrix background is a system baseplate fixed to the viewport and does not follow page scrolling, content scrolling, or fixed area mask movement.

### 4.1 Fixed Area

The fixed area includes:

* `SATURN` top banner
* Left `Control Rack`

Role of the fixed area:

* Carry system identity.
* Carry main module switching.
* Express system status.
* Serve visually as a chassis / rack.

Fixed area design rules:

* Use dimmer matte panels.
* Heavier borders can be used.
* Four small corner rivets can be used to express "fixed to the chassis".
* Do not use strong shadows.
* Do not use highlights brighter than the content area.

### 4.2 Scrollable Area

The scrollable area includes:

* Hero / title banner
* Module overview
* Recent objects
* Control references
* Color tokens
* Footer

Scrollable area design rules:

* Content scrolls normally with the page.
* Use regular surfaces, do not add fixed area rivets.
* Focus more on readability and component expression.
* Should not compete for hierarchy with the fixed area.

### 4.3 Top Mask

The top uses a background mask to crop scrollable content.

Rules:

* The bottom of the mask aligns with the top of the `Control Rack`.
* The mask reuses the page background without using hard dividing lines.
* The purpose is to prevent scrollable content from showing behind the fixed banner.

### 4.4 App Shell / Route Outlet

A real frontend implementation should not rewrite a complete page skeleton for every page.

SATURN should only have one `App Shell`, keeping the fixed area stable, with only the scrollable main content area switching based on routing. The main content area can be seen as a `route outlet` / `content outlet`.

```text
SATURN App Shell
├── Fixed Top Banner        Fixed
├── Fixed Control Rack      Fixed
└── Scrollable Main Area    Switches based on routing
    ├── Files Page
    ├── Notes Page
    ├── Accounting Page
    ├── Calendar Page
    ├── LLM Page
    └── Operations Page
```

Rules:

* The Top Banner and Control Rack belong to the App Shell and not to individual business pages.
* Route switching only replaces the module page content within the Scrollable Main Area.
* Files / Notes / Accounting / Calendar / LLM / Operations pages do not repeatedly declare global fixed areas.
* Cross-module capabilities such as global search, main module navigation, and system status are carried by the App Shell.
* The App Shell requests `/healthz` after authentication and every 5 seconds thereafter. A response with `status = ok` displays a green `Online` indicator; request failures and all other statuses display a red `Offline` indicator.
* The current static frontend first version uses a root-owned hash view: the browser request path remains `/`, the left Control Rack only updates hashes like `#files` / `#notes`, and switches the scrollable content area inside the root page.

## 5. Navigation Model

### 5.1 Desktop

The desktop uses a persistent `Control Rack`.

Main modules:

* Files
* Notes
* Accounting
* Calendar
* LLM
* Search
* Operations

The top `Global Search` is the input entry point for Ref IDs; the left `Search` is the query results page, and the content area of the first version only displays shared instance metadata JSON.

Reason:

* SATURN is a data console for long-term use, not a single-flow task application.
* Users will frequently switch between Files / Notes / Accounting / Calendar / LLM.
* Persistent menus better fit the usage patterns of backends, IDEs, data systems, and consoles.

### 5.2 Mobile

Mobile does not force a persistent left menu.

Suggested strategy:

* The Control Rack can be changed to a drawer or horizontal tabs.
* Top search can exclusively occupy a row.
* Tables can be downgraded to list cards.
* Fixed areas can revert to normal document flow to avoid obstruction.

### 5.3 Back Button

Back should not be used as a global primary navigation mode.

Recommended rules:

* Use the Control Rack for main module switching.
* Use local Back for detail pages, edit pages, and preview pages.
* Back appears in the Module Header / Detail Header in the right content area, rather than in the SATURN banner.
* Local return button text uses `RETURN`, not long sentences like `Back to ...`.

Example:

```text
Files
└── File Detail
    └── RETURN

Notes
└── Markdown Editor
    └── RETURN

LLM
└── Draft Preview
    └── RETURN
```

## 6. Product Modules

### 6.1 Search

Location: The top banner provides an input box, and the Control Rack provides a `Search` results page entry.

Capabilities:

* Exact query of shared instance metadata by `ref_code`
* JSON viewer responses contain the same object's `title`, `tags`, and `status`; `tags` for tagless objects is `[]`
* Supports `NTE` / `FIL` / `ACC` / `CAL` / `LLM` module prefixes
* The results page displays the API response in the mutable content area as a JSON viewer
* Does not redirect to business detail pages
* Does not provide full-text search

The first version `Search` is a display landing point for global input and does not own any source business rules.

### 6.2 Files

Content:

* File upload
* File collections
* Download
* Single file delete
* Collection cascade delete

Key components:

* Dropzone
* Collection List
* File List
* File Metadata Panel
* Download / Delete Controls

### 6.3 Notes

Content:

* Note list
* Markdown editing
* Tags
* Collections
* Templates
* Note references
* RSS sources

Key components:

* Markdown Editor
* Markdown Preview
* Tag Input
* Collection Picker
* Template Picker
* Linked References Panel

The current Notes page in `web/src` uses the shared instance `/api/notes` logical Note API: list/detail show the current Markdown snapshot, while every save creates an immutable `version-obj` and advances the stable `nte-obj` current pointer. The editor's `HISTORY` mode loads the current Note's version summaries and lets the user click a version to read its immutable content through the version RefCode endpoint without replacing the current draft. Delete permanently removes the logical Note, its complete version history, and all associated ObjectRefs; neither the UI nor API provides content restoration. Collections, templates, associations, and RSS remain future UI capabilities.

### 6.4 Accounting

Content:

* Ledger accounts
* Immutable transaction streams
* Transaction tags
* Cached balances
* Void operations

Key components:

* Account List
* Transaction Table
* Amount Cell
* Account Selector
* Tag Input
* Void Action

Note: Account is the ledger; Transactions can only be added or voided; deleting a ledger cascades the deletion of its transactions. Accounting only performs personal bookkeeping and balance statistics, not investment advice, financial advice, or sensitive financial analysis.

The current Accounting page in `web/src` is integrated with the main `/api/accounting` closed loop: the overview reads all accessible ledgers and current month transactions, displaying balances and current month income/expenses excluding voided transactions by currency; the list displays ledger categories using tags; after entering a single ledger, you can view paginated transactions, create new posted transactions, read immutable details and execute void, or you can create new ledgers with tags or delete ledgers. Since the current API does not provide Account modification or arbitrary Transaction editing endpoints, the page does not display fake edit or status save controls.

### 6.5 Calendar

Content:

* EventAggregate
* Event
* Main Calendar view
* None / week / month / year recurrence expansion by count upon Event creation

First version strategy:

* First implement aggregate lists, empty aggregate creation, aggregate details, event creation under aggregate, event finish, and event void operations.
* Create Event Aggregate also supports optional synchronous ICS import: the browser can fetch an HTTP(S) ICS URL into an editable text area, or the user can paste ICS text directly; submitting non-empty ICS text uses the existing multipart import endpoint, while an empty text area keeps the normal empty aggregate flow.
* Do not rush into complex calendar views.
* Small monthly calendars and agenda previews can be retained in the current component reference page for visual reference.

Key components:

* EventAggregate Row
* Event Row
* Void Action
* Date Picker
* Time Picker
* End Date / Time Input
* Recurrence Kind / Count Controls
* Schedule / Agenda List

### 6.6 LLM

Content:

* Ask My Data
* Summarize files / notes
* Generate drafts
* Tag suggestions
* Reference suggestions
* Result polling

First version boundaries:

* read-only
* draft-only
* Does not automatically execute dangerous operations

Key components:

* Ask Panel
* Answer View
* Source Reference List
* Draft Preview
* Mode Badge

### 6.7 Operations

Content:

* Storage status
* Audit entries
* Account controls

Principles:

> The Ops UI is an aggregation layer and does not reimplement business logic.

Key components:

* Storage Meter
* Audit Log Table
* System Health Panel
* Account Control Panel

The current operations page in `web/src` uses a selector-first structure. The root view only chooses between Audit and Account; Audit opens the audit filter, table, and selected event detail; Account opens current principal detail plus create-account, change-password, and logout operation pages. These views reuse Platform Auth and Audit APIs rather than adding independent Ops business paths.

## 7. Component Inventory

Basic components currently existing in DESIGN HTML:

* App Shell
* SATURN Top Banner
* Control Rack
* Global Search
* Status Pill
* Status Light
* Module Card
* Data Table
* Button
* Chip
* Status Badge
* Input Group
* Input
* Select
* Textarea
* Meter / Progress
* Notice / Alert
* Pagination
* Slider
* Switch
* Calendar / Schedule
* System Clock
* Color Tokens

Suggested future additions:

* Empty State
* Loading State
* Error State
* Modal / Dialog
* Drawer / Side Panel
* Tabs
* Filter Bar
* Breadcrumb
* Dropzone
* Markdown Editor
* Reference Panel
* Transaction Table Variant
* Repeat Rule Builder
* LLM Ask Panel
* Source References
* Draft Preview
* Job Table
* Audit Log

## 8. Interaction Rules

### 8.1 Buttons

* Primary buttons use orange.
* Secondary buttons use surface backgrounds.
* Danger buttons use red.
* Flat / Chip / Status types do not use inset shadows.
* Fixed area navigation does not use depression displacements.
* Page-level confirm, save, delete, return, etc. action buttons should be placed at the bottom right of the content area if possible; inline list actions can be placed at the end of the row.
* Multi-button groups prioritize right alignment, with the primary action on the far right, maintaining clear spacing between dangerous and regular operations.
* Button text uses simple, straightforward uppercase English command words, prioritizing single words, without writing descriptive phrases.
* The common vocabulary prioritizes `NEW` / `SAVE` / `EDIT` / `VIEW` / `VOID` / `RETURN` / `EXPORT` / `RESTORE` / `IMPORT` / `UPLOAD` / `DOWNLOAD` / `SEARCH` / `OPEN` / `RUN` / `LOAD` / `WAIT` / `LOGIN` / `LOGOUT`.
* Creating resources uses `NEW`, not long sentences like `Create Object`, `Add New ...`.
* Voiding, reversing, or canceling business documents or transactions uses `VOID`; `DELETE` is only used for permanent deletion when the business semantics genuinely mean delete.
* Local returns use `RETURN`, not long sentences like `Back to Files`, `Back to ...`.
* Avoid descriptive button text like `Click here to save`, `View Architecture`, `Run Index`; context is supplied by page titles, form titles, adjacent instructions, or `aria-label`.

### 8.2 Control Rack

* Navigation items are switchable but do not cause page jumps.
* The current item uses `aria-current="page"`.
* A status light is on the left of each row.
* The default light is gray; the current item is green.
* Visually more like a rack selector than a standard button list.

### 8.3 Switch

Switches use a standard structure:

* The track is a semi-circular rectangle.
* The slider is a circle.
* The 'on' state track is orange.
* The 'off' state track is beige / default surface.
* Depression feedback is not used, only toggling is expressed.

### 8.4 Clock

The System Clock is placed below the Control Rack.

Purpose:

* Enhance the feeling of a running system.
* Reinforce the status panel attribute of the fixed area.

Rules:

* Use monospace numbers.
* Seconds can be displayed to make the system look live.
* Should not be more conspicuous than navigation.

### 8.5 Calendar

The Calendar component serves as a visual reference for the Calendar module.

Rules:

* The first version does not strive for a fully complex calendar.
* Small monthly calendars can be used for date selection.
* Agenda lists are used to display scheduled Events.
* Small green dots can be used for dates with events.

## 9. Accessibility Rules

* Global search uses a real `form` and `input type="search"`.
* Navigation switching uses `button type="button"`, avoiding `href="#"` causing page jumps.
* The current navigation item uses `aria-current="page"`.
* Switches use `role="switch"` and `aria-checked"`.
* Progress uses `role="progressbar"`, `aria-valuemin`, `aria-valuemax`, `aria-valuenow`.
* Calendar date buttons use `aria-selected` to identify the currently selected date.
* Important status notifications can use `role="status"`.
* Support `:focus-visible`.
* Support `prefers-reduced-motion`.

## 10. File Placement

Recommended file locations:

```text
docs/
├── DESIGN.md
└── DESIGN.html
```

If later split into a real frontend project, it can be migrated to:

```text
web/
├── src/
│   ├── app/
│   ├── components/
│   ├── modules/
│   └── styles/
└── docs/
    ├── DESIGN.md
    └── DESIGN.html
```

## 11. Implementation Notes

The current `DESIGN.html` is a single-file prototype suitable for:

* Visual direction calibration
* Component reference
* Design boilerplate prior to frontend implementation
* Accompanying PRD / ER / code layout documents

When moving to real implementation, splitting is recommended:

```text
components/
├── app-shell
├── control-rack
├── global-search
├── status
├── forms
├── table
├── calendar
├── clock
├── llm
└── ops
```

CSS is recommended to retain a token-first structure:

* color tokens
* typography tokens
* spacing tokens
* radius tokens
* density / layout semantic tokens
* base primitives
* component styles
* responsive rules

Current location of the shared frontend library in `web/src`:

```text
web/src/shared/components
├── primitives.js          # surface / section / button / status / card / table / notice / meter
├── module_placeholder.js  # module placeholder pages not connected to real business flows
└── system_clock.js        # Control Rack system clock

web/src/shared/layout
├── app_shell.js           # Top Banner + Control Rack + scrollable main composition
├── control_rack.js        # Main module selector
├── global_search.js       # Top global search form
└── top_banner.js          # SATURN banner and system status

web/src/shared/utils
└── markdown.js            # marked parse + DOMPurify sanitize for Note preview

web/src/vendor
├── marked.min.js          # backend-served fixed browser runtime
└── purify.min.js          # backend-served fixed browser runtime
```

These files remain plain DOM/HTML without introducing frontend frameworks, bundlers, CSS frameworks, or third-party component libraries. The Note body is uniformly Markdown; the frontend HTML preview must be sanitized by DOMPurify before being written to the DOM.

## 12. Design Principle

> Retro-futurism is only for ambiance; actual implementation relies on clear grids, solid borders, monospace identifiers, low-saturation backgrounds, readable data tables, and distinct fixed/scrollable hierarchies.
