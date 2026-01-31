// Package report génère des rapports HTML interactifs pour visualiser les résultats du scan.
//
// CONCEPT : GÉNÉRATION HTML AVEC html/template
// =============================================
// Go dispose d'un package html/template qui permet de générer du HTML de façon sécurisée.
// Les templates utilisent une syntaxe spéciale : {{.Variable}} pour insérer des valeurs.
//
// AVANTAGES :
// - Échappement automatique des caractères spéciaux (protection XSS)
// - Séparation claire entre logique Go et présentation HTML
// - Possibilité de définir des fonctions personnalisées
package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"FileRecoveryOrganizer/dedup"
	"FileRecoveryOrganizer/types"
	"FileRecoveryOrganizer/utils"
)

// ═══════════════════════════════════════════════════════════════════════════
// TYPES POUR LE RAPPORT
// ═══════════════════════════════════════════════════════════════════════════

// ReportData contient toutes les données pour générer le rapport HTML.
type ReportData struct {
	// Informations générales
	Title        string    `json:"title"`
	GeneratedAt  time.Time `json:"generated_at"`
	SourceDir    string    `json:"source_dir"`
	ScanDuration string    `json:"scan_duration"`

	// Statistiques globales
	TotalFiles     int    `json:"total_files"`
	TotalDirs      int    `json:"total_dirs"`
	TotalSize      int64  `json:"total_size"`
	TotalSizeHuman string `json:"total_size_human"`

	// Répartition par catégorie
	Categories []CategoryStats `json:"categories"`

	// Répartition par type de fichier
	FileTypes []FileTypeStats `json:"file_types"`

	// Liste des fichiers (limitée pour le HTML)
	Files []FileEntry `json:"files"`

	// Doublons (si calculés)
	HasDuplicates   bool                   `json:"has_duplicates"`
	DuplicateReport *dedup.DuplicateReport `json:"duplicate_report,omitempty"`
}

// CategoryStats représente les statistiques d'une catégorie.
type CategoryStats struct {
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	Size       int64   `json:"size"`
	SizeHuman  string  `json:"size_human"`
	Percentage float64 `json:"percentage"`
}

// FileTypeStats représente les statistiques d'un type de fichier.
type FileTypeStats struct {
	Extension  string  `json:"extension"`
	Count      int     `json:"count"`
	Size       int64   `json:"size"`
	SizeHuman  string  `json:"size_human"`
	Percentage float64 `json:"percentage"`
}

// FileEntry représente un fichier dans le rapport.
type FileEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	SizeHuman  string `json:"size_human"`
	Category   string `json:"category"`
	TargetPath string `json:"target_path"`
	HasDate    bool   `json:"has_date"`
	Date       string `json:"date,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS DE GÉNÉRATION
// ═══════════════════════════════════════════════════════════════════════════

// GenerateHTML génère un rapport HTML à partir des résultats du scan.
//
// Paramètres :
//   - results : liste des fichiers scannés
//   - sourceDir : répertoire source scanné
//   - outputPath : chemin du fichier HTML de sortie
//   - duplicates : rapport de doublons (peut être nil)
//
// Retour :
//   - error : erreur si la génération échoue
func GenerateHTML(results []types.Result, sourceDir string, outputPath string, duplicates *dedup.DuplicateReport) error {
	// Construire les données du rapport
	data := buildReportData(results, sourceDir, duplicates)

	// Créer le fichier de sortie
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("impossible de créer le fichier HTML: %w", err)
	}
	defer f.Close()

	// Parser et exécuter le template
	tmpl, err := template.New("report").Funcs(templateFuncs()).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("erreur de parsing du template: %w", err)
	}

	return tmpl.Execute(f, data)
}

// buildReportData construit les données du rapport à partir des résultats.
func buildReportData(results []types.Result, sourceDir string, duplicates *dedup.DuplicateReport) ReportData {
	data := ReportData{
		Title:       "File Recovery Organizer - Rapport",
		GeneratedAt: time.Now(),
		SourceDir:   sourceDir,
		TotalFiles:  len(results),
	}

	// Maps pour accumuler les statistiques
	categoryStats := make(map[string]*CategoryStats)
	typeStats := make(map[string]*FileTypeStats)

	// Parcourir tous les résultats
	for _, r := range results {
		data.TotalSize += r.Size

		// Extraire la catégorie du TargetPath
		category := extractCategory(r.TargetPath)

		// Statistiques par catégorie
		if _, ok := categoryStats[category]; !ok {
			categoryStats[category] = &CategoryStats{Name: category}
		}
		categoryStats[category].Count++
		categoryStats[category].Size += r.Size

		// Statistiques par type
		ext := r.Type
		if ext == "" {
			ext = "unknown"
		}
		if _, ok := typeStats[ext]; !ok {
			typeStats[ext] = &FileTypeStats{Extension: ext}
		}
		typeStats[ext].Count++
		typeStats[ext].Size += r.Size
	}

	// Convertir les maps en slices triées
	data.Categories = mapToCategories(categoryStats, data.TotalFiles)
	data.FileTypes = mapToFileTypes(typeStats, data.TotalFiles)

	// Limiter les fichiers pour le HTML (les 1000 premiers)
	maxFiles := 1000
	if len(results) < maxFiles {
		maxFiles = len(results)
	}
	data.Files = make([]FileEntry, maxFiles)
	for i := 0; i < maxFiles; i++ {
		r := results[i]
		data.Files[i] = FileEntry{
			Path:       r.Path,
			Name:       filepath.Base(r.Path),
			Type:       r.Type,
			Size:       r.Size,
			SizeHuman:  utils.ReadableSize(r.Size),
			Category:   extractCategory(r.TargetPath),
			TargetPath: r.TargetPath,
		}
		if r.AdditionalInfo != nil && r.AdditionalInfo.Valid {
			data.Files[i].HasDate = true
			data.Files[i].Date = r.AdditionalInfo.Time.Format("2006-01-02")
		}
	}

	data.TotalSizeHuman = utils.ReadableSize(data.TotalSize)

	// Doublons
	if duplicates != nil && duplicates.DuplicateGroups > 0 {
		data.HasDuplicates = true
		data.DuplicateReport = duplicates
	}

	return data
}

// extractCategory extrait la catégorie principale du chemin cible.
func extractCategory(targetPath string) string {
	if targetPath == "" {
		return "Non classé"
	}
	// Le premier segment du chemin est la catégorie
	parts := filepath.SplitList(targetPath)
	if len(parts) > 0 {
		// Sur Windows, filepath.SplitList ne fait pas ce qu'on veut
		// Utilisons filepath.Split à la place
		dir := filepath.Dir(targetPath)
		for dir != "." && dir != "" && filepath.Dir(dir) != dir {
			parent := filepath.Dir(dir)
			if parent == "." || parent == "" {
				return filepath.Base(dir)
			}
			dir = parent
		}
		return filepath.Base(targetPath)
	}
	return "Non classé"
}

// mapToCategories convertit la map en slice triée par count décroissant.
func mapToCategories(m map[string]*CategoryStats, total int) []CategoryStats {
	result := make([]CategoryStats, 0, len(m))
	for _, v := range m {
		v.SizeHuman = utils.ReadableSize(v.Size)
		if total > 0 {
			v.Percentage = float64(v.Count) * 100 / float64(total)
		}
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

// mapToFileTypes convertit la map en slice triée par count décroissant.
func mapToFileTypes(m map[string]*FileTypeStats, total int) []FileTypeStats {
	result := make([]FileTypeStats, 0, len(m))
	for _, v := range m {
		v.SizeHuman = utils.ReadableSize(v.Size)
		if total > 0 {
			v.Percentage = float64(v.Count) * 100 / float64(total)
		}
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	// Limiter à 50 types pour le graphique
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// templateFuncs retourne les fonctions disponibles dans le template.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"readableSize": utils.ReadableSize,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TEMPLATE HTML
// ═══════════════════════════════════════════════════════════════════════════

const htmlTemplate = `<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        :root {
            --primary: #3b82f6;
            --primary-dark: #2563eb;
            --success: #22c55e;
            --warning: #f59e0b;
            --danger: #ef4444;
            --bg: #0f172a;
            --bg-card: #1e293b;
            --bg-hover: #334155;
            --text: #f1f5f9;
            --text-muted: #94a3b8;
            --border: #334155;
        }
        
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', system-ui, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }
        
        header {
            text-align: center;
            margin-bottom: 2rem;
            padding-bottom: 2rem;
            border-bottom: 1px solid var(--border);
        }
        
        h1 {
            font-size: 2.5rem;
            margin-bottom: 0.5rem;
            background: linear-gradient(135deg, var(--primary), #8b5cf6);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        
        .meta {
            color: var(--text-muted);
            font-size: 0.9rem;
        }
        
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        
        .stat-card {
            background: var(--bg-card);
            border-radius: 12px;
            padding: 1.5rem;
            border: 1px solid var(--border);
        }
        
        .stat-card .label {
            color: var(--text-muted);
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        
        .stat-card .value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--primary);
            margin-top: 0.25rem;
        }
        
        .stat-card.warning .value { color: var(--warning); }
        .stat-card.danger .value { color: var(--danger); }
        .stat-card.success .value { color: var(--success); }
        
        .section {
            background: var(--bg-card);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 2rem;
            border: 1px solid var(--border);
        }
        
        .section h2 {
            font-size: 1.25rem;
            margin-bottom: 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .charts-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
            gap: 2rem;
        }
        
        .chart-container {
            position: relative;
            height: 300px;
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
        }
        
        th, td {
            padding: 0.75rem 1rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }
        
        th {
            background: var(--bg);
            font-weight: 600;
            color: var(--text-muted);
            font-size: 0.85rem;
            text-transform: uppercase;
            position: sticky;
            top: 0;
        }
        
        tr:hover td {
            background: var(--bg-hover);
        }
        
        .file-path {
            max-width: 400px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            font-family: 'Consolas', monospace;
            font-size: 0.85rem;
        }
        
        .badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
        }
        
        .badge-primary { background: var(--primary); }
        .badge-success { background: var(--success); }
        .badge-warning { background: var(--warning); color: #000; }
        .badge-danger { background: var(--danger); }
        
        .search-box {
            width: 100%;
            padding: 0.75rem 1rem;
            background: var(--bg);
            border: 1px solid var(--border);
            border-radius: 8px;
            color: var(--text);
            font-size: 1rem;
            margin-bottom: 1rem;
        }
        
        .search-box:focus {
            outline: none;
            border-color: var(--primary);
        }
        
        .table-container {
            max-height: 500px;
            overflow-y: auto;
        }
        
        .duplicate-group {
            background: var(--bg);
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1rem;
        }
        
        .duplicate-group h4 {
            color: var(--warning);
            margin-bottom: 0.5rem;
        }
        
        .duplicate-group ul {
            list-style: none;
            font-family: 'Consolas', monospace;
            font-size: 0.85rem;
        }
        
        .duplicate-group li {
            padding: 0.25rem 0;
            color: var(--text-muted);
        }
        
        .progress-bar {
            height: 8px;
            background: var(--bg);
            border-radius: 4px;
            overflow: hidden;
            margin-top: 0.5rem;
        }
        
        .progress-bar .fill {
            height: 100%;
            background: var(--primary);
            border-radius: 4px;
        }
        
        @media (max-width: 768px) {
            .container { padding: 1rem; }
            .charts-grid { grid-template-columns: 1fr; }
            h1 { font-size: 1.75rem; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>📁 File Recovery Organizer</h1>
            <p class="meta">
                Généré le {{.GeneratedAt.Format "02/01/2006 à 15:04:05"}}<br>
                Source: <code>{{.SourceDir}}</code>
            </p>
        </header>
        
        <!-- Statistiques globales -->
        <div class="stats-grid">
            <div class="stat-card">
                <div class="label">📄 Fichiers</div>
                <div class="value">{{.TotalFiles}}</div>
            </div>
            <div class="stat-card">
                <div class="label">💾 Taille totale</div>
                <div class="value">{{.TotalSizeHuman}}</div>
            </div>
            <div class="stat-card">
                <div class="label">📂 Catégories</div>
                <div class="value">{{len .Categories}}</div>
            </div>
            <div class="stat-card">
                <div class="label">📋 Types</div>
                <div class="value">{{len .FileTypes}}</div>
            </div>
            {{if .HasDuplicates}}
            <div class="stat-card warning">
                <div class="label">🔄 Doublons</div>
                <div class="value">{{.DuplicateReport.DuplicateFiles}}</div>
            </div>
            <div class="stat-card danger">
                <div class="label">💸 Espace gaspillé</div>
                <div class="value">{{readableSize .DuplicateReport.WastedSpace}}</div>
            </div>
            {{end}}
        </div>
        
        <!-- Graphiques -->
        <div class="charts-grid">
            <div class="section">
                <h2>📊 Répartition par catégorie</h2>
                <div class="chart-container">
                    <canvas id="categoryChart"></canvas>
                </div>
            </div>
            <div class="section">
                <h2>📈 Types de fichiers (Top 15)</h2>
                <div class="chart-container">
                    <canvas id="typeChart"></canvas>
                </div>
            </div>
        </div>
        
        <!-- Tableau des catégories -->
        <div class="section">
            <h2>📁 Détail par catégorie</h2>
            <table>
                <thead>
                    <tr>
                        <th>Catégorie</th>
                        <th>Fichiers</th>
                        <th>Taille</th>
                        <th>Pourcentage</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Categories}}
                    <tr>
                        <td><strong>{{.Name}}</strong></td>
                        <td>{{.Count}}</td>
                        <td>{{.SizeHuman}}</td>
                        <td>
                            <div class="progress-bar">
                                <div class="fill" style="width: {{printf "%.1f" .Percentage}}%"></div>
                            </div>
                            {{printf "%.1f" .Percentage}}%
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        
        {{if .HasDuplicates}}
        <!-- Section Doublons -->
        <div class="section">
            <h2>🔄 Fichiers en double</h2>
            <p style="color: var(--text-muted); margin-bottom: 1rem;">
                {{.DuplicateReport.DuplicateGroups}} groupes de doublons détectés, 
                représentant {{readableSize .DuplicateReport.WastedSpace}} d'espace gaspillé.
            </p>
            {{range $i, $group := .DuplicateReport.Groups}}
            {{if lt $i 20}}
            <div class="duplicate-group">
                <h4>Groupe {{$i}} - {{$group.Count}} fichiers ({{readableSize $group.Size}} chacun)</h4>
                <ul>
                    {{range $j, $path := $group.Paths}}
                    {{if lt $j 5}}
                    <li>{{$path}}</li>
                    {{end}}
                    {{end}}
                    {{if gt (len $group.Paths) 5}}
                    <li>... et {{len $group.Paths | printf "%d"}} autres fichiers</li>
                    {{end}}
                </ul>
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}
        
        <!-- Tableau des fichiers -->
        <div class="section">
            <h2>📄 Liste des fichiers ({{len .Files}} affichés sur {{.TotalFiles}})</h2>
            <input type="text" class="search-box" id="searchBox" placeholder="🔍 Rechercher un fichier...">
            <div class="table-container">
                <table id="filesTable">
                    <thead>
                        <tr>
                            <th>Nom</th>
                            <th>Type</th>
                            <th>Taille</th>
                            <th>Catégorie</th>
                            <th>Date</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Files}}
                        <tr>
                            <td class="file-path" title="{{.Path}}">{{.Name}}</td>
                            <td><span class="badge badge-primary">{{.Type}}</span></td>
                            <td>{{.SizeHuman}}</td>
                            <td>{{.Category}}</td>
                            <td>{{if .HasDate}}{{.Date}}{{else}}-{{end}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
    
    <script>
        // Données pour les graphiques
        const categories = {{json .Categories}};
        const fileTypes = {{json .FileTypes}};
        
        // Couleurs pour les graphiques
        const colors = [
            '#3b82f6', '#8b5cf6', '#ec4899', '#ef4444', '#f59e0b',
            '#22c55e', '#14b8a6', '#06b6d4', '#6366f1', '#a855f7',
            '#f43f5e', '#84cc16', '#eab308', '#0ea5e9', '#d946ef'
        ];
        
        // Graphique des catégories (Doughnut)
        new Chart(document.getElementById('categoryChart'), {
            type: 'doughnut',
            data: {
                labels: categories.map(c => c.name),
                datasets: [{
                    data: categories.map(c => c.count),
                    backgroundColor: colors.slice(0, categories.length),
                    borderWidth: 0
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right',
                        labels: { color: '#f1f5f9' }
                    }
                }
            }
        });
        
        // Graphique des types (Bar - Top 15)
        const top15Types = fileTypes.slice(0, 15);
        new Chart(document.getElementById('typeChart'), {
            type: 'bar',
            data: {
                labels: top15Types.map(t => t.extension),
                datasets: [{
                    label: 'Nombre de fichiers',
                    data: top15Types.map(t => t.count),
                    backgroundColor: '#3b82f6',
                    borderRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                indexAxis: 'y',
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: {
                        grid: { color: '#334155' },
                        ticks: { color: '#94a3b8' }
                    },
                    y: {
                        grid: { display: false },
                        ticks: { color: '#f1f5f9' }
                    }
                }
            }
        });
        
        // Recherche dans le tableau
        document.getElementById('searchBox').addEventListener('input', function(e) {
            const search = e.target.value.toLowerCase();
            const rows = document.querySelectorAll('#filesTable tbody tr');
            rows.forEach(row => {
                const text = row.textContent.toLowerCase();
                row.style.display = text.includes(search) ? '' : 'none';
            });
        });
    </script>
</body>
</html>`
