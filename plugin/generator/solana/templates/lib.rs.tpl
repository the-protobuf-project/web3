{{.Header}}
use anchor_lang::prelude::*;

declare_id!("{{.ProgramID}}");

#[program]
pub mod {{.Program}} {
    use super::*;
{{range .Accounts}}
    /// Create a new {{.Struct}} record at its PDA. Fails if the account exists.
    pub fn create_{{.Snake}}(ctx: Context<Create{{.Struct}}>, data: {{.Struct}}Data) -> Result<()> {
        let record = &mut ctx.accounts.{{.Snake}};
{{- range .Fields}}
        record.{{.Name}} = data.{{.Name}};
{{- end}}
{{- if .Access}}
        record.owner = ctx.accounts.authority.key();
{{- end}}
        Ok(())
    }

    /// Overwrite an existing {{.Struct}} record.
    pub fn update_{{.Snake}}(ctx: Context<Update{{.Struct}}>, data: {{.Struct}}Data) -> Result<()> {
        let record = &mut ctx.accounts.{{.Snake}};
{{- if .Access}}
        require_keys_eq!(record.owner, ctx.accounts.authority.key(), ChainError::Unauthorized);
{{- end}}
{{- range .Fields}}
        record.{{.Name}} = data.{{.Name}};
{{- end}}
        Ok(())
    }

    /// Close a {{.Struct}} record and refund its rent to the authority.
    pub fn close_{{.Snake}}(ctx: Context<Close{{.Struct}}>) -> Result<()> {
{{- if .Access}}
        require_keys_eq!(ctx.accounts.{{.Snake}}.owner, ctx.accounts.authority.key(), ChainError::Unauthorized);
{{- end}}
        Ok(())
    }
{{end}}}
{{range .Accounts}}
/// {{.Comment}}
#[account]
#[derive(InitSpace)]
pub struct {{.Struct}} {
{{- range .Fields}}
{{- if .Comment}}
    /// {{.Comment}}
{{- end}}
{{- if .MaxLen}}
    #[max_len({{.MaxLen}})]
{{- end}}
    pub {{.Name}}: {{.Type}},
{{- end}}
{{- if .Access}}
    /// The account authorized to update or close this record.
    pub owner: Pubkey,
{{- end}}
}

/// AnchorSerialize payload carrying every {{.Struct}} field for create/update.
#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct {{.Struct}}Data {
{{- range .Fields}}
    pub {{.Name}}: {{.Type}},
{{- end}}
}

#[derive(Accounts)]
#[instruction(data: {{.Struct}}Data)]
pub struct Create{{.Struct}}<'info> {
    #[account(
        init,
        payer = authority,
        space = 8 + {{.Struct}}::INIT_SPACE,
        seeds = [b"{{.Snake}}", {{.KeySeed}}],
        bump,
    )]
    pub {{.Snake}}: Account<'info, {{.Struct}}>,
    #[account(mut)]
    pub authority: Signer<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
#[instruction(data: {{.Struct}}Data)]
pub struct Update{{.Struct}}<'info> {
    #[account(mut, seeds = [b"{{.Snake}}", {{.KeySeed}}], bump)]
    pub {{.Snake}}: Account<'info, {{.Struct}}>,
    pub authority: Signer<'info>,
}

#[derive(Accounts)]
pub struct Close{{.Struct}}<'info> {
    #[account(mut, close = authority)]
    pub {{.Snake}}: Account<'info, {{.Struct}}>,
    #[account(mut)]
    pub authority: Signer<'info>,
}
{{end}}
{{- range .Enums}}
/// {{.Comment}}
#[derive(AnchorSerialize, AnchorDeserialize, Clone, Copy, PartialEq, Eq, InitSpace)]
pub enum {{.Name}} {
{{- range .Values}}
{{- if .Comment}}
    /// {{.Comment}}
{{- end}}
    {{.Name}},
{{- end}}
}
{{end}}
#[error_code]
pub enum ChainError {
    #[msg("caller is not authorized to modify this record")]
    Unauthorized,
}
