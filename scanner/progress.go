package scanner

import "github.com/schollz/progressbar/v3"

// CreateProgessBar crée une barre de progression pour afficher l'avancement du scan.
//
// CONCEPT : Feedback utilisateur en temps réel
// =============================================
// Quand on traite un grand nombre de fichiers, l'utilisateur veut voir
// l'avancement en temps réel. Une barre de progression affiche :
// - Nombre de fichiers traités / total
// - Pourcentage complété
// - Vitesse de traitement (fichiers par seconde)
// - Temps restant estimé
//
// EXEMPLE D'AFFICHAGE :
// 45% |████████░░░░░░░░░░| (300/672947, 88 it/s) [5s:2h7m0s]
//
//	  ↑                  ↑          ↑           ↑  ↑
//	Barre          Compteur     Vitesse    Temps écoulé:estimé
//
// PARAMÈTRES :
//   - total : nombre total de fichiers à traiter
//     obtenu en général avec CountFile() avant de lancer ScanDirectoryParallel
//
// UTILISATION :
// Cette fonction est appelée dans les options d'initialisation du scan.
// Elle retourne un *progressbar.ProgressBar qui peut être passé en callback
// via la fonction barUpdate à ScanDirectoryParallel.
//
// Retour :
//   - *progressbar.ProgressBar : pointeur vers la barre (utilisable plusieurs fois)
func CreateProgessBar(total int) *progressbar.ProgressBar {
	return progressbar.Default(int64(total))
}
