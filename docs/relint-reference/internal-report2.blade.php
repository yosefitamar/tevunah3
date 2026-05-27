<html>

<head>
    <link rel="stylesheet" href="{{ resource_path('/css/internal-report.css') }}">
</head>

<body>
    <div class="fpheader">
        <div class="spacer"></div>
        <div class="fpborder">
            &nbsp;
        </div>

        <table class="data-table">
            <tr>
                <th>DATA</th>
                <td>: {{ $diffusionDate }}</td>
            </tr>
            <tr>
                <th>ASSUNTO</th>
                <td>: {{ $report->subject }}</td>
            </tr>
            <tr>
                <th>ORIGEM</th>
                <td>: SAI 2ºBPRAIO/ASINT/PMCE</td>
            </tr>
            <tr>
                <th>DIFUSÃO</th>
                <td>: {{ $report->diffusion_level ?? 'INTERNA' }}</td>
            </tr>
            <tr>
                <th>DIF. ANT</th>
                <td>: {{ $report->previous_diffusion ?? '*.*.*' }}</td>
            </tr>
            <tr>
                <th>REF</th>
                <td>:
                    @php
                        $refIds = $report->reference;

                        if (is_array($refIds) && count($refIds) > 0) {
                            $referencedReports = \App\Models\InternalReport::whereIn('id', $refIds)->get();

                            $links = $referencedReports->map(function ($r) {
                                $url = asset('relatorios/' . $r->filename);
                                $termo = $r->termo;

                                return '<a href="' . $url . '" target="_blank" rel="noopener noreferrer">' . e($termo) . '</a>';
                            })->all();

                            $ref = count($links) ? implode(', ', $links) : '*.*.*';
                        } else {
                            $ref = '*.*.*';
                        }
                    @endphp

                    {!! $ref !!}
                </td>
            </tr>
            <tr>
                <th>ANEXO(S)</th>
                <td>: {{ $report->attachments ?? '*.*.*' }}</td>
            </tr>
        </table>

        <hr>
    </div>

    <header>
        <div class="header">
            <div class="sigilo">S E C R E T O</div>
            <div class="logos">
                <table style="width: 100%; border: none;">
                    <tr>
                        <td style="text-align: left; width: 20%;">
                            <img src="data:image/png;base64, {!! $qrcode !!}" alt="QR Code do Relatório">
                        </td>

                        <td style="text-align: left; width: 55%;">
                            <img src="{{ resource_path('img/logo-instituicao2.png') }}" alt="Logo Instituição"
                                style="max-height: 2cm;">
                        </td>

                        <td style="text-align: right; width: 25%;">
                            <img src="{{ resource_path('img/logo-sai.jpg') }}" alt="Logo SAI" style="max-height: 2cm;">
                        </td>
                    </tr>
                </table>

                <div class="title">
                    {{$term}}
                </div>
            </div>
        </div>
    </header>

    <footer>
        <div class="footer">

            <div class="warningborder">
                <p class="message">
                    "O sigilo deste documento é protegido e controlado pela Lei no 12.527/2011. A divulgação, a
                    revelação, o
                    fornecimento, a
                    utilização ou a reprodução desautorizada de seu conteúdo, a qualquer tempo, meio ou modo,
                    inclusive
                    mediante acesso ou
                    facilitação de acesso indevidos, constituem condutas ilícitas que ensejam responsabilidades
                    penais,
                    civis e
                    administrativas."
                </p>

                <p class="message2">
                    É vedada a retransmissão deste documento
                </p>
            </div>

            <div class="sigilo">S E C R E T O</div>

            <div class="footermark">
                <img src="{{ resource_path('img/footer.png') }}" alt="">
            </div>
        </div>
    </footer>

    <main style="margin-bottom: 40px">
        <div class="content">
            {!! $report->content_with_absolute_image_paths !!}
        </div>

        @if ($report->reportSuspects->count() > 0)
            <div class="page-break"></div>

            <div class="qualifications">
                <h2>QUALIFICAÇÕES</h2>
                @foreach ($report->reportSuspects as $suspect)
                    <table class="suspect-table" border="1" cellpadding="0" cellspacing="0" align="center">
                        <thead>
                            <tr>
                                <th colspan="3" class="suspect-header">{{ $suspect->suspect->full_name }}</th>
                            </tr>
                        </thead>
                        <tbody style="border: 1px solid">
                            <tr>
                                <td rowspan="6" class="suspect-image-cell">
                                    @if ($suspect->suspect->profile_picture_path)
                                        <img src="{{ storage_path('/app/public/' . $suspect->suspect->profile_picture_path) }}"
                                            alt="Foto do Suspeito" class="suspect-photo">
                                    @else
                                        <img src="{{ resource_path('img/photo-placeholder.png') }}" alt="Foto não disponível"
                                            class="suspect-photo">
                                    @endif
                                </td>
                                <td class="label">ALCUNHA</td>
                                <td class="label2">{{ $suspect->suspect->alias ?? "*.*.*" }}</td>
                            </tr>
                            <tr>
                                <td class="label">CPF</td>
                                <td class="label2">{{ $suspect->suspect->cpf ?? "*.*.*" }}</td>
                            </tr>
                            <tr>
                                <td class="label">ORCRIM</td>
                                <td class="label2">{{ $suspect->suspect->orcrim ?? "*.*.*" }}</td>
                            </tr>
                            <tr>
                                <td class="label">ENDEREÇOS</td>
                                <td class="label2">{!! $suspect->suspect->addresses ?? "*.*.*" !!}</td>
                            </tr>
                            <tr>
                                <td class="label">VEÍCULOS</td>
                                <td class="label2">{!! $suspect->suspect->vehicles ?? "*.*.*" !!}</td>
                            </tr>
                            <tr>
                                <td class="label">INFORMAÇÕES<br> ADICIONAIS</td>
                                <td class="label2">{!! $suspect->suspect->additional_information ?? "*.*.*" !!}</td>
                            </tr>
                        </tbody>
                    </table>
                @endforeach
            </div>
        @endif
        <div class="end_doc">*.*.*</div>
    </main>
</body>

</html>